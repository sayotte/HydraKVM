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
	"sort"
	"sync"
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
// DefaultChannel, when non-nil, is auto-attached to every Client passed to
// [Application.AddClient]. Wiring code (cmd/hydrakvm) must set it before
// any client is registered; mutating it concurrently with AddClient is not
// supported.
type Application struct {
	baseCtx context.Context

	// DefaultChannel is attached to each new Client by AddClient when set.
	DefaultChannel *Channel

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
}

type channelHandle struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// NewApplication returns a fresh, empty Application. Channel goroutines
// derive their context from baseCtx; cancelling baseCtx tears down every
// active Channel.
func NewApplication(baseCtx context.Context) *Application {
	return &Application{
		baseCtx:        baseCtx,
		channels:       make(map[string]*Channel),
		clientChan:     make(map[*Client]*Channel),
		channelClients: make(map[*Channel]map[*Client]struct{}),
		running:        make(map[*Channel]*channelHandle),
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

// AddClient registers c. If [Application.DefaultChannel] is non-nil, c is
// immediately attached to it (ref-counting that channel's goroutine), so that
// new Clients can exercise the key/video paths without an explicit
// SwitchChannel call.
func (a *Application) AddClient(c *Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clientChan[c] = nil
	if a.DefaultChannel != nil {
		a.attachLocked(c, a.DefaultChannel)
	}
}

// ChannelInfo describes one registered Channel for enumeration by the HTTP
// layer. Channel is exported so callers can resolve the *Channel for fan-out
// without taking the Application lock.
type ChannelInfo struct {
	ID      string
	Channel *Channel
}

// Channels returns the registered Channels in stable ID order, omitting the
// reserved "__default__" channel which is not user-selectable.
func (a *Application) Channels() []ChannelInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ids := make([]string, 0, len(a.channels))
	for id := range a.channels {
		if id == defaultChannelID {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ChannelInfo, len(ids))
	for i, id := range ids {
		out[i] = ChannelInfo{ID: id, Channel: a.channels[id]}
	}
	return out
}

// defaultChannelID is the reserved registry key under which wiring code
// stores [Application.DefaultChannel] so it benefits from the standard
// AddChannel path while remaining hidden from user-facing channel lists.
const defaultChannelID = "__default__"

// RemoveClient detaches c from any Channel and forgets it. If c was the
// last Client on its Channel, the Channel's goroutine is stopped.
func (a *Application) RemoveClient(c *Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
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
// p.ChannelID, ref-counting Channel goroutines as attachments change.
func (a *Application) SwitchChannel(ctx context.Context, p SwitchChannelParams) (SwitchChannelResult, error) {
	c := ClientFromContext(ctx)
	if c == nil {
		return SwitchChannelResult{}, ErrNoClient
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ch, ok := a.channels[p.ChannelID]
	if !ok {
		return SwitchChannelResult{}, fmt.Errorf("%w: %q", ErrUnknownChannel, p.ChannelID)
	}
	if _, known := a.clientChan[c]; !known {
		return SwitchChannelResult{}, ErrUnknownClient
	}
	if a.clientChan[c] == ch {
		return SwitchChannelResult(p), nil
	}
	a.detachLocked(c)
	a.attachLocked(c, ch)
	return SwitchChannelResult(p), nil
}

// RecordKeyEvent forwards ke to the Channel currently attached to the Client
// in ctx. Send blocks until the Channel's drainer accepts it (or ctx is
// cancelled); backpressure from a slow sink propagates upstream to the
// dispatcher's caller.
func (a *Application) RecordKeyEvent(ctx context.Context, ke KeyEvent) error {
	c := ClientFromContext(ctx)
	if c == nil {
		return ErrNoClient
	}
	a.mu.RLock()
	ch, known := a.clientChan[c]
	a.mu.RUnlock()
	if !known {
		return ErrUnknownClient
	}
	if ch == nil {
		return ErrNoActiveChannel
	}
	return ch.SendKeyEvent(ctx, ke)
}

// attachLocked adds c to ch's client set, starting ch's goroutine if c is
// the first attached Client. Caller must hold a.mu.
func (a *Application) attachLocked(c *Client, ch *Channel) {
	set, ok := a.channelClients[ch]
	if !ok {
		set = make(map[*Client]struct{})
		a.channelClients[ch] = set
	}
	if len(set) == 0 {
		a.startChannelLocked(ch)
	}
	set[c] = struct{}{}
	a.clientChan[c] = ch
}

// detachLocked removes c from its current Channel (if any), stopping the
// Channel's goroutine if c was the last attached Client. Caller must hold
// a.mu.
func (a *Application) detachLocked(c *Client) {
	ch := a.clientChan[c]
	if ch == nil {
		return
	}
	set := a.channelClients[ch]
	delete(set, c)
	a.clientChan[c] = nil
	if len(set) == 0 {
		delete(a.channelClients, ch)
		a.stopChannelLocked(ch)
	}
}

func (a *Application) startChannelLocked(ch *Channel) {
	ctx, cancel := context.WithCancel(a.baseCtx)
	h := &channelHandle{cancel: cancel, done: make(chan struct{})}
	a.running[ch] = h
	go func() {
		defer close(h.done)
		ch.Run(ctx)
	}()
}

func (a *Application) stopChannelLocked(ch *Channel) {
	h, ok := a.running[ch]
	if !ok {
		return
	}
	delete(a.running, ch)
	h.cancel()
}
