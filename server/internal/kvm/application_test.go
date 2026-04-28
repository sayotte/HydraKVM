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
	"testing"
)

func TestSwitchChannelRequiresClient(t *testing.T) {
	app := NewApplication(t.Context())
	app.AddChannel("ch1", NewChannel(nil, nil))

	_, err := app.SwitchChannel(t.Context(), SwitchChannelParams{ChannelID: "ch1"})
	if !errors.Is(err, ErrNoClient) {
		t.Errorf("got %v want ErrNoClient", err)
	}
}

func TestSwitchChannelUnknownChannel(t *testing.T) {
	app := NewApplication(t.Context())
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	_, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "missing"})
	if !errors.Is(err, ErrUnknownChannel) {
		t.Errorf("got %v want ErrUnknownChannel", err)
	}
}

func TestSwitchChannelUnknownClient(t *testing.T) {
	app := NewApplication(t.Context())
	app.AddChannel("ch1", NewChannel(nil, nil))
	c := &Client{} // not registered
	ctx := WithClient(t.Context(), c)

	_, err := app.SwitchChannel(ctx, SwitchChannelParams{ChannelID: "ch1"})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient", err)
	}
}

func TestSwitchChannelSucceeds(t *testing.T) {
	app := NewApplication(t.Context())
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
	app := NewApplication(t.Context())
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
	app := NewApplication(t.Context())
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
	app := NewApplication(t.Context())
	c := &Client{}
	app.AddClient(c)
	ctx := WithClient(t.Context(), c)

	err := app.RecordKeyEvent(ctx, KeyEvent{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrNoActiveChannel) {
		t.Errorf("got %v want ErrNoActiveChannel", err)
	}
}

func TestRecordKeyEventUnknownClient(t *testing.T) {
	app := NewApplication(t.Context())
	c := &Client{} // not registered
	ctx := WithClient(t.Context(), c)

	err := app.RecordKeyEvent(ctx, KeyEvent{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient", err)
	}
}

func TestAddClientAutoAttachesToDefaultChannel(t *testing.T) {
	app := NewApplication(t.Context())
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

func TestApplicationChannelsListsRegisteredExceptDefault(t *testing.T) {
	app := NewApplication(t.Context())
	chDef := NewChannel(nil, nil)
	chA := NewChannel(nil, nil)
	chB := NewChannel(nil, nil)
	app.AddChannel("__default__", chDef)
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

func TestRemoveClient(t *testing.T) {
	app := NewApplication(t.Context())
	c := &Client{}
	app.AddClient(c)
	app.RemoveClient(c)

	ctx := WithClient(t.Context(), c)
	err := app.RecordKeyEvent(ctx, KeyEvent{Code: KeyA, Type: KeyTypeDown})
	if !errors.Is(err, ErrUnknownClient) {
		t.Errorf("got %v want ErrUnknownClient after RemoveClient", err)
	}
}
