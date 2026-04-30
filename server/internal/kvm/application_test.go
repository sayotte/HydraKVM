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

func TestAddClientAutoAttachesToDefaultChannel(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	ch := NewChannel(nil, nil)
	app.AddChannel("__default__", ch)
	app.DefaultChannel = ch

	c := &Client{}
	app.AddClient(c)

	if got := app.ChannelOf(c); got != ch {
		t.Errorf("ChannelOf: got %p want %p", got, ch)
	}
	if !app.IsChannelRunning(ch) {
		t.Error("expected default channel to be running after AddClient")
	}
}

func TestApplicationChannelsListsRegisteredIncludingDefault(t *testing.T) {
	app := NewApplication(t.Context(), nil)
	chDef := NewChannel(nil, nil)
	chA := NewChannel(nil, nil)
	chB := NewChannel(nil, nil)
	app.AddChannel(DefaultChannelID, chDef)
	app.AddChannel("b", chB)
	app.AddChannel("a", chA)

	got := app.Channels()
	if len(got) != 3 {
		t.Fatalf("Channels len: got %d want 3", len(got))
	}
	// Underscore (0x5F) sorts before lowercase letters, so the default
	// lands at index 0.
	if got[0].ID != DefaultChannelID || got[1].ID != "a" || got[2].ID != "b" {
		t.Errorf("Channels order: got %q,%q,%q want %q,a,b",
			got[0].ID, got[1].ID, got[2].ID, DefaultChannelID)
	}
	if got[0].Channel != chDef || got[1].Channel != chA || got[2].Channel != chB {
		t.Error("Channels did not return matching *Channel pointers")
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
