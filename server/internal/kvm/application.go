// Copyright (C) 2026 Stephen Ayotte
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package kvm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Errors returned by Application.
var (
	ErrNoClient        = errors.New("no client in context")
	ErrUnknownChannel  = errors.New("unknown channel")
	ErrUnknownClient   = errors.New("unknown client")
	ErrNoActiveChannel = errors.New("client has no active channel")
)

// Application owns all domain decisions: the Channel registry, the
// Client-to-Channel association, and the lifecycle of per-Channel goroutines.
// It is the single source of truth for which Clients are attached to which
// Channels — Clients are passive bags of sinks and do not carry a
// current-channel pointer themselves.
//
// Channel goroutines are reference-counted on attached Clients: a Channel's
// [Channel.Run] is launched when the first Client attaches and torn down
// (via the per-Channel context) when the last Client detaches. This ensures
// hardware FDs (HDMI capture, serial port) are released while no Client is
// watching, and gives Application a place to handle driver errors that the
// Webclient can't act on.
//
// NoChannelVideo, when non-nil, is the [VideoSource] Application pumps into
// each Client's FrameSink while the Client is unattached (has not yet
// selected a Channel, or has selected the empty channel id). It is
// conceptually distinct from FallbackVideo, which only fires when a real
// Channel's primary VideoIn fails. Wiring code (cmd/hydrakvm) supplies it
// at startup; mutating it concurrently with AddClient is not supported.
//
// FallbackVideo, when non-nil, is the [VideoSource] Application substitutes
// for any Channel whose primary VideoIn fails (Subscribe channel closes
// while Channel ctx is live). Application also pushes a [MsgClientUpdate]
// notification to every Client on the affected Channel, and continues
// probing the primary; on recovery it swaps back and notifies again.
type Application struct {
	baseCtx context.Context
	logger  *slog.Logger

	// NoChannelVideo feeds the per-Client video pipe whenever the Client is
	// unattached (post-AddClient, or after SwitchChannel with empty id).
	NoChannelVideo VideoSource

	// FallbackVideo is the source Application uses to keep a Channel's
	// frames flowing while its primary VideoSource is down.
	FallbackVideo VideoSource

	// VideoRecoveryProbeInterval is how often Application retries Subscribe
	// on a failed primary source. Zero means use the package default.
	VideoRecoveryProbeInterval time.Duration

	mu sync.RWMutex

	channels   map[string]*Channel
	clientChan map[*Client]*Channel

	// channelClients is the reverse of clientChan: per-Channel set of
	// attached clients. Used for ref-counted goroutine lifecycle and
	// (later) for fan-out of frame distribution and notifications.
	channelClients map[*Channel]map[*Client]struct{}

	// running tracks the per-Channel cancel/done pair for in-flight
	// goroutines. A Channel is in this map iff its Run goroutine is
	// currently active.
	running map[*Channel]*channelHandle

	// noChanPumps tracks the per-Client cancel func for the goroutine
	// pumping NoChannelVideo into that Client's FrameSink. A Client is in
	// this map iff a no-channel pump is currently active for it.
	noChanPumps map[*Client]context.CancelFunc
}

type channelHandle struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// defaultVideoRecoveryProbeInterval bounds Application's retry cadence after
// a primary VideoSource has failed. Set conservatively: ffmpeg respawn under
// the v4l driver is itself rate-limited at ~2s, and there is no benefit to
// probing faster than that.
const defaultVideoRecoveryProbeInterval = 2 * time.Second

// NewApplication returns a fresh, empty Application. Channel goroutines
// derive their context from baseCtx; cancelling baseCtx tears down every
// active Channel. If logger is nil, [slog.Default] is used.
func NewApplication(baseCtx context.Context, logger *slog.Logger) *Application {
	if logger == nil {
		logger = slog.Default()
	}
	return &Application{
		baseCtx:        baseCtx,
		logger:         logger,
		channels:       make(map[string]*Channel),
		clientChan:     make(map[*Client]*Channel),
		channelClients: make(map[*Channel]map[*Client]struct{}),
		running:        make(map[*Channel]*channelHandle),
		noChanPumps:    make(map[*Client]context.CancelFunc),
	}
}

// AddChannel registers ch under the given id. The Channel is not started
// here; Application launches its goroutine on first Client attach and stops
// it on last detach.
func (a *Application) AddChannel(id string, ch *Channel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.channels[id] = ch
}

// AddClient registers c in the unattached state. The Client's FrameSink is
// fed from [Application.NoChannelVideo] (when set) until the Client sends an
// explicit [MsgSwitchChannel] selecting a real Channel. No Channel is
// auto-attached and KbdState is not touched.
func (a *Application) AddClient(c *Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clientChan[c] = nil
	a.startNoChannelPumpLocked(c)
}

// ChannelInfo describes one registered Channel for enumeration by the HTTP
// layer. Channel is exported so callers can resolve the *Channel for fan-out
// without taking the Application lock.
type ChannelInfo struct {
	ID      string
	Channel *Channel
}

// Channels returns the registered Channels in stable ID order. The
// "(none)" / unattached state is not represented as a Channel; the HTTP
// layer renders it as a separate selector entry.
func (a *Application) Channels() []ChannelInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ids := make([]string, 0, len(a.channels))
	for id := range a.channels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ChannelInfo, len(ids))
	for i, id := range ids {
		out[i] = ChannelInfo{ID: id, Channel: a.channels[id]}
	}
	return out
}

// NoChannelID is the sentinel SwitchChannel ID that returns a Client to the
// unattached state. SwitchChannel with this id detaches the Client from any
// current Channel (without disturbing KbdState) and resumes feeding the
// Client's FrameSink from NoChannelVideo.
const NoChannelID = ""

// RemoveClient detaches c from any Channel and forgets it. If c was the
// last Client on its Channel, the Channel's goroutine is stopped. If c was
// unattached, its no-channel pump is stopped.
func (a *Application) RemoveClient(c *Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopNoChannelPumpLocked(c)
	a.detachLocked(c)
	delete(a.clientChan, c)
}

// ChannelOf returns the Channel currently attached to c, or nil.
func (a *Application) ChannelOf(c *Client) *Channel {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.clientChan[c]
}

// IsChannelRunning reports whether ch's goroutine is currently active.
// Intended for tests and observability; production code should not need it.
func (a *Application) IsChannelRunning(ch *Channel) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.running[ch]
	return ok
}

// SwitchChannel moves the Client carried in ctx onto the Channel named by
// p.ChannelID, ref-counting Channel goroutines as attachments change. The
// sentinel [NoChannelID] (empty string) detaches the Client and resumes the
// no-channel video pump without disturbing any KbdState. On a successful
// transition Application re-emits [MsgMJPEGURL] to the Client as a hint so
// the browser can re-attach its long-lived <img> source.
func (a *Application) SwitchChannel(ctx context.Context, p SwitchChannelParams) (SwitchChannelResult, error) {
	c := ClientFromContext(ctx)
	if c == nil {
		return SwitchChannelResult{}, ErrNoClient
	}
	a.mu.Lock()
	known := false
	if _, ok := a.clientChan[c]; ok {
		known = true
	}
	if !known {
		a.mu.Unlock()
		return SwitchChannelResult{}, ErrUnknownClient
	}

	if p.ChannelID == NoChannelID {
		if a.clientChan[c] == nil {
			a.mu.Unlock()
			return SwitchChannelResult(p), nil
		}
		a.detachLocked(c)
		a.startNoChannelPumpLocked(c)
		a.mu.Unlock()
		a.emitMJPEGURL(c)
		return SwitchChannelResult(p), nil
	}

	ch, ok := a.channels[p.ChannelID]
	if !ok {
		a.mu.Unlock()
		return SwitchChannelResult{}, fmt.Errorf("%w: %q", ErrUnknownChannel, p.ChannelID)
	}
	if a.clientChan[c] == ch {
		a.mu.Unlock()
		return SwitchChannelResult(p), nil
	}
	a.stopNoChannelPumpLocked(c)
	a.detachLocked(c)
	a.attachLocked(c, ch)
	a.mu.Unlock()
	a.emitMJPEGURL(c)
	return SwitchChannelResult(p), nil
}

// emitMJPEGURL re-sends the per-Client MJPEG stream URL as a hint after a
// successful channel transition. Callers must not hold a.mu — Outbound is a
// network sink.
func (a *Application) emitMJPEGURL(c *Client) {
	if c.Outbound == nil || c.MJPEGStreamURL == "" {
		return
	}
	if err := c.Outbound.WriteMessage(MsgMJPEGURL, MJPEGURLParams{URL: c.MJPEGStreamURL}); err != nil {
		a.logger.Warn("mjpeg url write failed", "client", clientID(c), "err", err)
	}
}

// startNoChannelPumpLocked launches a goroutine that pumps NoChannelVideo
// frames into c's FrameSink until cancelled. Caller must hold a.mu. No-op
// when NoChannelVideo is unset or a pump is already running for c.
func (a *Application) startNoChannelPumpLocked(c *Client) {
	if a.NoChannelVideo == nil {
		return
	}
	if _, running := a.noChanPumps[c]; running {
		return
	}
	ctx, cancel := context.WithCancel(a.baseCtx)
	a.noChanPumps[c] = cancel
	src := a.NoChannelVideo
	go func() {
		frames := src.Subscribe(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case vf, ok := <-frames:
				if !ok {
					return
				}
				if sink := c.VideoOut(); sink != nil {
					sink.WriteFrame(vf)
				}
			}
		}
	}()
}

// stopNoChannelPumpLocked cancels and clears any active no-channel pump for
// c. Caller must hold a.mu. Safe to call when no pump is active.
func (a *Application) stopNoChannelPumpLocked(c *Client) {
	cancel, ok := a.noChanPumps[c]
	if !ok {
		return
	}
	delete(a.noChanPumps, c)
	cancel()
}

// RecordKeyEvent enqueues the edge in p onto the Channel currently attached
// to the Client in ctx. The Channel's drainer applies the edge to KbdState
// and dispatches a fully-stamped KeyEvent in arrival order. Send blocks until
// the drainer accepts the edge (or ctx is cancelled); backpressure from a
// slow sink propagates upstream to the dispatcher's caller.
func (a *Application) RecordKeyEvent(ctx context.Context, p KeyEventParams) error {
	c := ClientFromContext(ctx)
	if c == nil {
		a.logger.Warn("record key event rejected", "reason", "no client", "code", p.Code, "type", p.Type)
		return ErrNoClient
	}
	a.mu.RLock()
	ch, known := a.clientChan[c]
	chID := ""
	if known && ch != nil {
		chID = a.channelIDLocked(ch)
	}
	a.mu.RUnlock()
	if !known {
		a.logger.Warn("record key event rejected", "reason", "unknown client",
			"client", clientID(c), "code", p.Code, "type", p.Type)
		return ErrUnknownClient
	}
	if ch == nil {
		a.logger.Warn("record key event rejected", "reason", "no active channel",
			"client", clientID(c), "code", p.Code, "type", p.Type)
		return ErrNoActiveChannel
	}
	if err := ch.SendKeyEdge(ctx, p.Code, p.Type); err != nil {
		a.logger.Warn("record key event rejected", "reason", "send failed",
			"channel", chID, "client", clientID(c), "code", p.Code, "type", p.Type, "err", err)
		return err
	}
	a.logger.Debug("record key event",
		"channel", chID, "client", clientID(c), "code", p.Code, "type", p.Type)
	return nil
}

// attachLocked adds c to ch's client set, starting ch's goroutine if c is
// the first attached Client. Caller must hold a.mu.
func (a *Application) attachLocked(c *Client, ch *Channel) {
	set, ok := a.channelClients[ch]
	if !ok {
		set = make(map[*Client]struct{})
		a.channelClients[ch] = set
	}
	first := len(set) == 0
	if first {
		a.startChannelLocked(ch)
	}
	set[c] = struct{}{}
	a.clientChan[c] = ch
	ch.RegisterClient(c)
	if first {
		a.logger.Info("channel attached",
			"channel", a.channelIDLocked(ch), "client", clientID(c))
	}
}

// detachLocked removes c from its current Channel (if any), stopping the
// Channel's goroutine if c was the last attached Client. Caller must hold
// a.mu.
func (a *Application) detachLocked(c *Client) {
	ch := a.clientChan[c]
	if ch == nil {
		return
	}
	ch.UnregisterClient(c)
	set := a.channelClients[ch]
	delete(set, c)
	a.clientChan[c] = nil
	if len(set) == 0 {
		delete(a.channelClients, ch)
		a.stopChannelLocked(ch)
		a.logger.Info("channel detached",
			"channel", a.channelIDLocked(ch), "client", clientID(c))
	}
}

// channelIDLocked returns the registered ID for ch, or "" if not registered.
// Caller must hold a.mu.
func (a *Application) channelIDLocked(ch *Channel) string {
	for id, c := range a.channels {
		if c == ch {
			return id
		}
	}
	return ""
}

func clientID(c *Client) string {
	return fmt.Sprintf("%p", c)
}

func (a *Application) startChannelLocked(ch *Channel) {
	ctx, cancel := context.WithCancel(a.baseCtx)
	h := &channelHandle{cancel: cancel, done: make(chan struct{})}
	a.running[ch] = h
	var wg sync.WaitGroup
	wg.Go(func() {
		ch.Run(ctx)
	})
	if ch.VideoIn != nil {
		wg.Go(func() {
			a.superviseVideo(ctx, ch)
		})
	}
	go func() {
		wg.Wait()
		close(h.done)
	}()
}

// superviseVideo subscribes to ch.VideoIn and pumps frames into ch.Fanout.
// If the subscription channel closes while ctx is live, Application swaps the
// fan-out to FallbackVideo (when set), notifies every attached Client with a
// redundant [MsgMJPEGURL] (the Client's own stream URL) followed by
// [MsgClientUpdate]/[ClientUpdateVideoDown], and starts a recovery probe.
// On the first frame received from a re-subscribe to the primary, the
// fallback pump is cancelled, fan-out resumes from the primary, and Clients
// receive [MsgMJPEGURL] followed by [ClientUpdateVideoRecovered]. The
// MJPEGURL re-emit is a hint for the browser to re-attach its long-lived
// <img> source if it dropped.
func (a *Application) superviseVideo(ctx context.Context, ch *Channel) {
	chID := a.channelID(ch)
	for {
		if !a.pumpFrames(ctx, ch.VideoIn, ch) {
			return
		}
		if ctx.Err() != nil {
			return
		}
		a.logger.Warn("channel video down", "channel", chID)
		a.notifyChannelClients(ch, ClientUpdateVideoDown, chID)
		if !a.runFallbackUntilRecovery(ctx, ch) {
			return
		}
		a.logger.Info("channel video recovered", "channel", chID)
		a.notifyChannelClients(ch, ClientUpdateVideoRecovered, chID)
	}
}

// pumpFrames subscribes to src and forwards frames to ch.Fanout until either
// ctx is cancelled (returns false) or src's channel closes (returns true,
// indicating a failure that the caller should react to).
func (a *Application) pumpFrames(ctx context.Context, src VideoSource, ch *Channel) bool {
	subCtx, subCancel := context.WithCancel(ctx)
	defer subCancel()
	frames := src.Subscribe(subCtx)
	for {
		select {
		case <-ctx.Done():
			return false
		case vf, ok := <-frames:
			if !ok {
				return true
			}
			ch.Fanout(vf)
		}
	}
}

// runFallbackUntilRecovery pumps from FallbackVideo (if set) into ch.Fanout
// while periodically probing ch.VideoIn for recovery. Returns true on
// recovery, false if ctx is cancelled. If FallbackVideo is nil the channel
// simply has no frames during the outage; recovery probing still runs.
func (a *Application) runFallbackUntilRecovery(ctx context.Context, ch *Channel) bool {
	fallbackCtx, fallbackCancel := context.WithCancel(ctx)
	defer fallbackCancel()
	if a.FallbackVideo != nil {
		go func() {
			frames := a.FallbackVideo.Subscribe(fallbackCtx)
			for {
				select {
				case <-fallbackCtx.Done():
					return
				case vf, ok := <-frames:
					if !ok {
						return
					}
					ch.Fanout(vf)
				}
			}
		}()
	}
	interval := a.VideoRecoveryProbeInterval
	if interval <= 0 {
		interval = defaultVideoRecoveryProbeInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
		}
		if a.probePrimaryOnce(ctx, ch.VideoIn) {
			return true
		}
	}
}

// probePrimaryOnce subscribes to src and waits up to one probe interval for
// a single frame; if one arrives, the source is healthy. The subscription is
// always cancelled before returning.
func (a *Application) probePrimaryOnce(ctx context.Context, src VideoSource) bool {
	probeCtx, probeCancel := context.WithCancel(ctx)
	defer probeCancel()
	frames := src.Subscribe(probeCtx)
	interval := a.VideoRecoveryProbeInterval
	if interval <= 0 {
		interval = defaultVideoRecoveryProbeInterval
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	case _, ok := <-frames:
		return ok
	}
}

func (a *Application) notifyChannelClients(ch *Channel, status ClientUpdateStatus, chID string) {
	payload := ClientUpdateParams{ChannelID: chID, Status: status}
	ch.ForEachClient(func(c *Client) {
		if c.Outbound == nil {
			return
		}
		if c.MJPEGStreamURL != "" {
			if err := c.Outbound.WriteMessage(MsgMJPEGURL, MJPEGURLParams{URL: c.MJPEGStreamURL}); err != nil {
				a.logger.Warn("mjpeg url write failed",
					"channel", chID, "client", clientID(c), "status", status, "err", err)
			}
		}
		if err := c.Outbound.WriteMessage(MsgClientUpdate, payload); err != nil {
			a.logger.Warn("client update write failed",
				"channel", chID, "client", clientID(c), "status", status, "err", err)
		}
	})
}

func (a *Application) channelID(ch *Channel) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.channelIDLocked(ch)
}

func (a *Application) stopChannelLocked(ch *Channel) {
	h, ok := a.running[ch]
	if !ok {
		return
	}
	delete(a.running, ch)
	h.cancel()
}
