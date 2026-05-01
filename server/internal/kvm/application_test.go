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
	"sync"
	"testing"
	"time"
)

func TestSwitchChannelRequiresClient(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	app.AddChannel("ch1", NewChannel(nil, nil))

	_, err := app.SwitchChannel(t.Context(), SwitchChannelParams{ChannelID: "ch1"})
	if !errors.Is(err, ErrNoClient) {
		t.Errorf("got %v want ErrNoClient", err)
	}
}

func TestSwitchChannelUnknownChannel(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	_, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "missing"})
	if !errors.Is(err, ErrUnknownChannel) {
		t.Errorf("got %v want ErrUnknownChannel", err)
	}
}

func TestSwitchChannelUnknownClient(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	app.AddChannel("ch1", NewChannel(nil, nil))
	c := &Client{} // not registered
	ctx := WithClient(t.Context(), c)

	_, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "ch1"})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient", err)
	}
}

func TestSwitchChannelSucceeds(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	ch := NewChannel(nil, nil)
	app.AddChannel("ch1", ch)
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	res, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "ch1"})
	if err != nil {
		t.Fatalf("SwitchChannel: %v", err)
	}
	if res.ChannelID != "ch1" {
		t.Errorf("result ChannelID: got %q want %q", res.ChannelID, "ch1")
	}
	if got := app.ChannelOf(c); got != ch {
		t.Errorf("ChannelOf: got %p want %p", got, ch)
	}
	if !app.IsChannelRunning(ch) {
		t.Error("expected channel to be running after attach")
	}
}

// TestChannelLifecycleRefCounted verifies that a Channel's goroutine starts
// on first attach and stops on last detach, with multiple attaches keeping
// it alive.
func TestChannelLifecycleRefCounted(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	ch := NewChannel(nil, nil)
	app.AddChannel("ch1", ch)

	cA := &Client{}
	cB := &Client{}
	app.AddClient(cA)
	app.AddClient(cB)

	if app.IsChannelRunning(ch) {
		t.Fatal("channel should not be running before any client attaches")
	}

	if _, err := app.SwitchChannel(WithClient(t.Context(), cA), SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cA: %v", err)
	}
	if !app.IsChannelRunning(ch) {
		t.Fatal("channel should be running after first attach")
	}

	if _, err := app.SwitchChannel(WithClient(t.Context(), cB), SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cB: %v", err)
	}
	if !app.IsChannelRunning(ch) {
		t.Fatal("channel should remain running with two clients")
	}

	app.RemoveClient(cA)
	if !app.IsChannelRunning(ch) {
		t.Fatal("channel should remain running after one of two clients detaches")
	}

	app.RemoveClient(cB)
	if app.IsChannelRunning(ch) {
		t.Error("channel should be stopped after last client detaches")
	}
}

// TestSwitchChannelStopsOldStartsNew verifies a Client moving from one
// Channel to another stops the previous (when it was the only attached
// client) and starts the new.
func TestSwitchChannelStopsOldStartsNew(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	chA := NewChannel(nil, nil)
	chB := NewChannel(nil, nil)
	app.AddChannel("a", chA)
	app.AddChannel("b", chB)
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "a"}); err != nil {
		t.Fatalf("switch to a: %v", err)
	}
	if !app.IsChannelRunning(chA) || app.IsChannelRunning(chB) {
		t.Fatal("after switch to a: only chA should be running")
	}

	if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "b"}); err != nil {
		t.Fatalf("switch to b: %v", err)
	}
	if app.IsChannelRunning(chA) || !app.IsChannelRunning(chB) {
		t.Errorf("after switch to b: chA running=%v chB running=%v",
			app.IsChannelRunning(chA), app.IsChannelRunning(chB))
	}
}

func TestRecordKeyEventNoActiveChannel(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	err := app.RecordKeyEvent(ctx, KeyEventParams{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrNoActiveChannel) {
		t.Errorf("got %v want ErrNoActiveChannel", err)
	}
}

func TestRecordKeyEventUnknownClient(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	c := &Client{} // not registered
	ctx := WithClient(t.Context(), c)

	err := app.RecordKeyEvent(ctx, KeyEventParams{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient", err)
	}
}

func TestAddClientDoesNotAutoAttach(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	ch := NewChannel(nil, nil)
	app.AddChannel("ch1", ch)

	c := &Client{}
	app.AddClient(c)

	if got := app.ChannelOf(c); got != nil {
		t.Errorf("ChannelOf: got %p want nil", got)
	}
	if app.IsChannelRunning(ch) {
		t.Error("registered channel should not be running before any client attaches")
	}
}

func TestAddClientPumpsNoChannelVideoIntoSink(t *testing.T) {
	src := &fakeVideoSource{interval: 5 * time.Millisecond}
	app := NewApplication(t.Context(), nil)
	app.NoChannelVideo = src

	sink := &countingSink{}
	c := &Client{}
	c.SetVideoOut(sink)
	app.AddClient(c)
	defer app.RemoveClient(c)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sink.n.Load() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("no-channel pump delivered no frames (got %d)", sink.n.Load())
}

func TestAddClientEmitsNoFailureNotifications(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	app.NoChannelVideo = &fakeVideoSource{interval: 5 * time.Millisecond}

	writer := &recordingMessageWriter{}
	c := &Client{Outbound: writer}
	app.AddClient(c)
	defer app.RemoveClient(c)

	time.Sleep(50 * time.Millisecond)
	for _, m := range writer.snapshot() {
		if m.Type == MsgClientUpdate {
			cp, ok := m.Payload.(ClientUpdateParams)
			if ok && (cp.Status == ClientUpdateVideoDown || cp.Status == ClientUpdateVideoRecovered) {
				t.Errorf("unexpected %q on connect", cp.Status)
			}
		}
	}
}

func TestApplicationChannelsListsRegistered(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	chA := NewChannel(nil, nil)
	chB := NewChannel(nil, nil)
	app.AddChannel("b", chB)
	app.AddChannel("a", chA)

	got := app.Channels()
	if len(got) != 2 {
		t.Fatalf("Channels len: got %d want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("Channels order: got %q,%q want a,b", got[0].ID, got[1].ID)
	}
	if got[0].Channel != chA || got[1].Channel != chB {
		t.Error("Channels did not return matching *Channel pointers")
	}
}

// TestSwitchChannelToNoneDetaches: an attached Client switching to the empty
// id detaches from the Channel; subsequent frames come from NoChannelVideo,
// not the original Channel; KbdState on the original Channel is untouched.
func TestSwitchChannelToNoneDetaches(t *testing.T) {
	primary := &fakeVideoSource{interval: 5 * time.Millisecond}
	noChan := &fakeVideoSource{interval: 5 * time.Millisecond}
	ch := NewChannel(primary, nil)
	ch.KbdState.Modifiers = ModLeftShift // canary

	app := NewApplication(t.Context(), nil)
	app.NoChannelVideo = noChan
	app.AddChannel("ch1", ch)

	writer := &recordingMessageWriter{}
	const streamURL = "/stream/tok"
	c := &Client{Outbound: writer, MJPEGStreamURL: streamURL}
	c.SetVideoOut(&countingSink{})
	app.AddClient(c)
	defer app.RemoveClient(c)
	ctx := WithClient(t.Context(), c)

	if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if !app.IsChannelRunning(ch) {
		t.Fatal("channel should be running after attach")
	}

	if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: NoChannelID}); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if got := app.ChannelOf(c); got != nil {
		t.Errorf("ChannelOf after detach: got %p want nil", got)
	}
	if app.IsChannelRunning(ch) {
		t.Error("channel should not be running after last client detached via NoChannelID")
	}
	if ch.KbdState.Modifiers != ModLeftShift {
		t.Errorf("KbdState mutated by detach: got %#x want %#x",
			ch.KbdState.Modifiers, ModLeftShift)
	}

	// Re-aim the sink at a fresh counter so we can prove subsequent frames
	// are arriving from NoChannelVideo (the only live source feeding this
	// Client now).
	postSink := &countingSink{}
	c.SetVideoOut(postSink)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if postSink.n.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if postSink.n.Load() == 0 {
		t.Error("no NoChannelVideo frames after detach")
	}

	// MsgMJPEGURL re-emit on detach.
	saw := false
	for _, m := range writer.snapshot() {
		if m.Type != MsgMJPEGURL {
			continue
		}
		up, ok := m.Payload.(MJPEGURLParams)
		if ok && up.URL == streamURL {
			saw = true
			break
		}
	}
	if !saw {
		t.Error("expected MsgMJPEGURL re-emit after detach")
	}
}

func TestSwitchChannelNoneToNoneNoOp(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	app.NoChannelVideo = &fakeVideoSource{interval: 5 * time.Millisecond}
	c := &Client{}
	app.AddClient(c)
	defer app.RemoveClient(c)
	ctx := WithClient(t.Context(), c)

	for range 3 {
		if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: NoChannelID}); err != nil {
			t.Fatalf("none->none: %v", err)
		}
	}
}

func TestAttachRegistersClientForVideo(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	src := &fakeVideoSource{interval: 5 * time.Millisecond}
	ch := NewChannel(src, nil)
	app.AddChannel("ch1", ch)

	sink := &countingSink{}
	c := &Client{}
	c.SetVideoOut(sink)
	app.AddClient(c)

	if _, err := app.SwitchChannel(WithClient(t.Context(), c), SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("SwitchChannel: %v", err)
	}
	defer app.RemoveClient(c)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sink.n.Load() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("client received no frames after attach (got %d)", sink.n.Load())
}

func TestDetachUnregistersClientFromVideo(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	src := &fakeVideoSource{interval: 5 * time.Millisecond}
	ch := NewChannel(src, nil)
	app.AddChannel("ch1", ch)

	sink := &countingSink{}
	c := &Client{}
	c.SetVideoOut(sink)
	app.AddClient(c)
	if _, err := app.SwitchChannel(WithClient(t.Context(), c), SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("SwitchChannel: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if sink.n.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	app.RemoveClient(c)
	time.Sleep(50 * time.Millisecond)
	before := sink.n.Load()
	time.Sleep(50 * time.Millisecond)
	after := sink.n.Load()
	if after != before {
		t.Errorf("client received %d frames after detach", after-before)
	}
}

// TestRecordKeyEventStampsModifiers walks the Alt+A example from the Wave 2A
// spec: a modifier-down sets its bit, a non-modifier edge inherits the held
// mask, and a modifier-up clears the bit. Each outbound KeyEvent's Modifiers
// is the post-edge snapshot.
func TestRecordKeyEventStampsModifiers(t *testing.T) {
	sink := &recordingSink{}
	ch := NewChannel(nil, sink)

	app := NewApplication(t.Context(), nil)
	app.AddChannel("ch1", ch)
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)
	if _, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("SwitchChannel: %v", err)
	}
	defer app.RemoveClient(c)

	steps := []struct {
		in   KeyEventParams
		want KeyEvent
	}{
		{KeyEventParams{Code: AltLeft, Type: KeyTypeDown}, KeyEvent{Code: AltLeft, Type: KeyTypeDown, Modifiers: ModLeftAlt}},
		{KeyEventParams{Code: KeyA, Type: KeyTypeDown}, KeyEvent{Code: KeyA, Type: KeyTypeDown, Modifiers: ModLeftAlt}},
		{KeyEventParams{Code: KeyA, Type: KeyTypeUp}, KeyEvent{Code: KeyA, Type: KeyTypeUp, Modifiers: ModLeftAlt}},
		{KeyEventParams{Code: AltLeft, Type: KeyTypeUp}, KeyEvent{Code: AltLeft, Type: KeyTypeUp, Modifiers: 0}},
	}
	for i, s := range steps {
		if err := app.RecordKeyEvent(ctx, s.in); err != nil {
			t.Fatalf("step %d RecordKeyEvent: %v", i, err)
		}
	}

	deadline := time.Now().Add(time.Second)
	for {
		sink.mu.Lock()
		n := len(sink.events)
		sink.mu.Unlock()
		if n == len(steps) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d events; got %d", len(steps), n)
		}
		time.Sleep(time.Millisecond)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	for i, s := range steps {
		if sink.events[i] != s.want {
			t.Errorf("step %d: got %+v want %+v", i, sink.events[i], s.want)
		}
	}
	if got := ch.KbdState.Modifiers; got != 0 {
		t.Errorf("final KbdState.Modifiers = %#x, want 0", got)
	}
}

// TestRecordKeyEventConcurrentClientsModifierConsistency exercises two Clients
// driving the same Channel concurrently. The single drainer linearizes edges
// in chan-arrival order so that every emitted KeyEvent's Modifiers mask
// reflects the post-edge state at the moment that edge was processed.
func TestRecordKeyEventConcurrentClientsModifierConsistency(t *testing.T) {
	sink := &recordingSink{}
	ch := NewChannel(nil, sink)

	app := NewApplication(t.Context(), nil)
	app.AddChannel("ch1", ch)
	cA := &Client{}
	cB := &Client{}
	app.AddClient(cA)
	app.AddClient(cB)
	ctxA := WithClient(t.Context(), cA)
	ctxB := WithClient(t.Context(), cB)
	if _, err := app.SwitchChannel(ctxA, SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cA: %v", err)
	}
	if _, err := app.SwitchChannel(ctxB, SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cB: %v", err)
	}

	const reps = 100
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range reps {
			_ = app.RecordKeyEvent(ctxA, KeyEventParams{Code: ShiftLeft, Type: KeyTypeDown})
			_ = app.RecordKeyEvent(ctxA, KeyEventParams{Code: ShiftLeft, Type: KeyTypeUp})
		}
	}()
	go func() {
		defer wg.Done()
		for range reps {
			_ = app.RecordKeyEvent(ctxB, KeyEventParams{Code: ControlRight, Type: KeyTypeDown})
			_ = app.RecordKeyEvent(ctxB, KeyEventParams{Code: ControlRight, Type: KeyTypeUp})
		}
	}()
	wg.Wait()

	want := 4 * reps
	deadline := time.Now().Add(2 * time.Second)
	for {
		sink.mu.Lock()
		n := len(sink.events)
		sink.mu.Unlock()
		if n == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out: got %d events, want %d", n, want)
		}
		time.Sleep(time.Millisecond)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.maxSeen > 1 {
		t.Errorf("maxSeen concurrent ReportKeyEvent calls = %d, want 1", sink.maxSeen)
	}
	for i, ev := range sink.events {
		bit := ev.Code.ModifierBit()
		switch ev.Type {
		case KeyTypeDown:
			if ev.Modifiers&bit == 0 {
				t.Fatalf("event %d %+v: post-edge mask missing own bit %#x", i, ev, bit)
			}
		case KeyTypeUp:
			if ev.Modifiers&bit != 0 {
				t.Fatalf("event %d %+v: post-edge mask still has own bit %#x", i, ev, bit)
			}
		}
		if ev.Modifiers&^(ModLeftShift|ModRightCtrl) != 0 {
			t.Fatalf("event %d %+v: unexpected bits set in mask %#x", i, ev, ev.Modifiers)
		}
	}
	if ch.KbdState.Modifiers != 0 {
		t.Errorf("final KbdState.Modifiers = %#x, want 0", ch.KbdState.Modifiers)
	}
}

// failingVideoSource emits one frame, then closes its channel immediately,
// signalling source failure. Subsequent Subscribe calls behave the same way
// until Healthy is true, at which point a long-lived stream resumes.
type failingVideoSource struct {
	mu      sync.Mutex
	healthy bool
}

func (f *failingVideoSource) Shape() StreamShape     { return StreamShape{Codec: "mjpeg"} }
func (f *failingVideoSource) InitData() []byte       { return nil }
func (f *failingVideoSource) RequestKeyframe() error { return nil }
func (f *failingVideoSource) setHealthy(b bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthy = b
}
func (f *failingVideoSource) isHealthy() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.healthy
}
func (f *failingVideoSource) Subscribe(ctx context.Context) <-chan VideoFrame {
	ch := make(chan VideoFrame, 1)
	healthy := f.isHealthy()
	go func() {
		defer close(ch)
		if !healthy {
			return
		}
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				select {
				case ch <- VideoFrame{Data: []byte{0xCC}, IsKey: true}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

// recordingMessageWriter captures outbound messages from Application.
type recordingMessageWriter struct {
	mu       sync.Mutex
	messages []recordedMessage
}

type recordedMessage struct {
	Type    string
	Payload any
}

func (r *recordingMessageWriter) WriteMessage(msgType string, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, recordedMessage{Type: msgType, Payload: payload})
	return nil
}

func (r *recordingMessageWriter) snapshot() []recordedMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedMessage, len(r.messages))
	copy(out, r.messages)
	return out
}

// findFailoverPair returns the index of the MsgMJPEGURL message immediately
// followed by a ClientUpdateVideoDown notification, or -1 if not present.
func findFailoverPair(msgs []recordedMessage) int {
	for i := 0; i+1 < len(msgs); i++ {
		if msgs[i].Type != MsgMJPEGURL {
			continue
		}
		if msgs[i+1].Type != MsgClientUpdate {
			continue
		}
		cp, ok := msgs[i+1].Payload.(ClientUpdateParams)
		if ok && cp.Status == ClientUpdateVideoDown {
			return i
		}
	}
	return -1
}

func TestApplicationVideoFallbackAndRecovery(t *testing.T) {
	primary := &failingVideoSource{}
	fallback := &fakeVideoSource{interval: 5 * time.Millisecond}
	ch := NewChannel(primary, nil)

	app := NewApplication(t.Context(), nil)
	app.FallbackVideo = fallback
	app.VideoRecoveryProbeInterval = 30 * time.Millisecond
	app.AddChannel("ch1", ch)

	sink := &countingSink{}
	writer := &recordingMessageWriter{}
	const streamURL = "/stream/test-token"
	c := &Client{Outbound: writer, MJPEGStreamURL: streamURL}
	c.SetVideoOut(sink)
	app.AddClient(c)
	if _, err := app.SwitchChannel(WithClient(t.Context(), c), SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("SwitchChannel: %v", err)
	}
	defer app.RemoveClient(c)

	// SwitchChannel emits an MJPEGURL hint synchronously; skip past any
	// such pre-failover messages and locate the failover pair
	// (MsgMJPEGURL, MsgClientUpdate{video_down}) and at least one
	// fallback frame.
	failoverIdx := -1
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs := writer.snapshot()
		failoverIdx = findFailoverPair(msgs)
		if failoverIdx >= 0 && sink.n.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	msgs := writer.snapshot()
	if failoverIdx < 0 {
		t.Fatalf("did not see failover (MsgMJPEGURL, video_down) pair; messages = %+v", msgs)
	}
	if up, ok := msgs[failoverIdx].Payload.(MJPEGURLParams); !ok {
		t.Fatalf("failover msg payload type: got %T want MJPEGURLParams", msgs[failoverIdx].Payload)
	} else if up.URL != streamURL {
		t.Errorf("failover msg URL: got %q want %q", up.URL, streamURL)
	}
	p, ok := msgs[failoverIdx+1].Payload.(ClientUpdateParams)
	if !ok {
		t.Fatalf("failover msg[+1] payload type: got %T want ClientUpdateParams", msgs[failoverIdx+1].Payload)
	}
	if p.Status != ClientUpdateVideoDown {
		t.Errorf("failover status: got %q want %q", p.Status, ClientUpdateVideoDown)
	}
	if p.ChannelID != "ch1" {
		t.Errorf("failover channel id: got %q want %q", p.ChannelID, "ch1")
	}
	if sink.n.Load() == 0 {
		t.Error("expected fallback frames to reach sink")
	}

	// Recover: mark primary healthy. Application should detect on next probe
	// and emit (MsgMJPEGURL, MsgClientUpdate{video_recovered}) in that order.
	primary.setHealthy(true)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgs = writer.snapshot()
		for i, m := range msgs {
			cp, ok := m.Payload.(ClientUpdateParams)
			if !ok || cp.Status != ClientUpdateVideoRecovered {
				continue
			}
			if i == 0 {
				t.Fatalf("video_recovered not preceded by any prior message")
			}
			prev := msgs[i-1]
			if prev.Type != MsgMJPEGURL {
				t.Fatalf("recovery msg[i-1] type: got %q want %q", prev.Type, MsgMJPEGURL)
			}
			up, ok := prev.Payload.(MJPEGURLParams)
			if !ok {
				t.Fatalf("recovery msg[i-1] payload type: got %T want MJPEGURLParams", prev.Payload)
			}
			if up.URL != streamURL {
				t.Errorf("recovery msg[i-1] URL: got %q want %q", up.URL, streamURL)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("did not see video_recovered notification; messages = %+v", writer.snapshot())
}

func TestRemoveClient(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	c := &Client{}
	app.AddClient(c)
	app.RemoveClient(c)

	ctx := WithClient(t.Context(), c)
	err := app.RecordKeyEvent(ctx, KeyEventParams{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient after RemoveClient", err)
	}
}
