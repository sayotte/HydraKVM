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

package dispatch_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/dispatch"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

// recordingSink asserts at most one concurrent ReportKeyEvent call. The
// in-flight counter and event log share a mutex; the simulated work-time
// (delay) widens the window in which a concurrency bug would manifest.
type recordingSink struct {
	mu       sync.Mutex
	events   []kvm.KeyEvent
	inFlight int
	maxSeen  int
}

func (s *recordingSink) ReportKeyEvent(ke kvm.KeyEvent) {
	s.mu.Lock()
	s.inFlight++
	if s.inFlight > s.maxSeen {
		s.maxSeen = s.inFlight
	}
	s.mu.Unlock()

	// Simulate a multi-byte serial write.
	time.Sleep(50 * time.Microsecond)

	s.mu.Lock()
	s.events = append(s.events, ke)
	s.inFlight--
	s.mu.Unlock()
}

// TestDispatchInvokesApplicationMethod verifies the wiring: an inbound
// Envelope with Type=MsgSwitchChannel and a SwitchChannelParams payload
// reaches Application.SwitchChannel and mutates Application state.
func TestDispatchInvokesApplicationMethod(t *testing.T) {
	app := kvm.NewApplication(t.Context())
	ch := kvm.NewChannel(nil, nil)
	app.AddChannel("ch1", ch)
	c := &kvm.Client{}
	app.AddClient(c)

	r := dispatch.NewRouter()
	dispatch.Register(r, kvm.MsgSwitchChannel, app.SwitchChannel)

	payload, err := json.Marshal(kvm.SwitchChannelParams{ChannelID: "ch1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	ctx := kvm.WithClient(t.Context(), c)
	reply, err := r.Dispatch(ctx, dispatch.Envelope{
		Type:    kvm.MsgSwitchChannel,
		ID:      "req-1",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if reply == nil {
		t.Fatal("expected non-nil reply")
		return
	}
	if reply.ID != "req-1" {
		t.Errorf("reply ID: got %q want %q", reply.ID, "req-1")
	}

	var res kvm.SwitchChannelResult
	if err := json.Unmarshal(reply.Payload, &res); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}
	if res.ChannelID != "ch1" {
		t.Errorf("result ChannelID: got %q want %q", res.ChannelID, "ch1")
	}
	if got := app.ChannelOf(c); got != ch {
		t.Errorf("ChannelOf after dispatch: got %p want %p", got, ch)
	}
}

// TestTwoClientsConcurrentlyDriveOneChannelSerialized verifies the per-Channel
// serialization invariant: when two Clients attached to the same Channel
// concurrently fire MsgKeyEvent through the dispatcher, the underlying
// KeyEventSink sees exactly one ReportKeyEvent call at a time, all events
// arrive, and ordering is internally consistent (no missed or duplicated
// events).
func TestTwoClientsConcurrentlyDriveOneChannelSerialized(t *testing.T) {
	sink := &recordingSink{}
	ch := kvm.NewChannel(nil, sink)

	app := kvm.NewApplication(t.Context())
	app.AddChannel("ch1", ch)
	cA := &kvm.Client{}
	cB := &kvm.Client{}
	app.AddClient(cA)
	app.AddClient(cB)
	if _, err := app.SwitchChannel(kvm.WithClient(t.Context(), cA), kvm.SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cA: %v", err)
	}
	if _, err := app.SwitchChannel(kvm.WithClient(t.Context(), cB), kvm.SwitchChannelParams{ChannelID: "ch1"}); err != nil {
		t.Fatalf("attach cB: %v", err)
	}

	r := dispatch.NewRouter()
	// Until sub-step 2 lands the wire-side translator, the test routes a
	// post-translation kvm.KeyEvent through the dispatcher to exercise the
	// same Application path the real handler will use.
	dispatch.RegisterNotification(r, kvm.MsgKeyEvent, app.RecordKeyEvent)

	const eventsPerClient = 200
	send := func(ctx context.Context, code kvm.KeyCode) {
		payload, err := json.Marshal(kvm.KeyEvent{Code: code, Type: kvm.KeyTypeDown})
		if err != nil {
			t.Errorf("marshal: %v", err)
			return
		}
		_, err = r.Dispatch(ctx, dispatch.Envelope{
			Type:    kvm.MsgKeyEvent,
			Payload: payload,
		})
		if err != nil {
			t.Errorf("Dispatch: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	var sentA, sentB atomic.Int32
	go func() {
		defer wg.Done()
		ctx := kvm.WithClient(t.Context(), cA)
		for range eventsPerClient {
			send(ctx, kvm.KeyA)
			sentA.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		ctx := kvm.WithClient(t.Context(), cB)
		for range eventsPerClient {
			send(ctx, kvm.KeyB)
			sentB.Add(1)
		}
	}()
	wg.Wait()

	// Wait for the drainer to flush all events. The dispatcher only
	// returns after the unbuffered send completes, so the drainer has read
	// each event by then but may still be inside ReportKeyEvent for the
	// last one.
	deadline := time.Now().Add(2 * time.Second)
	want := 2 * eventsPerClient
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
		t.Errorf("maxSeen concurrent ReportKeyEvent calls = %d, want 1 (sink writes were not serialized)", sink.maxSeen)
	}
	var nA, nB int
	for _, ev := range sink.events {
		switch ev.Code {
		case kvm.KeyA:
			nA++
		case kvm.KeyB:
			nB++
		default:
			t.Errorf("unexpected event code %d", ev.Code)
		}
	}
	if nA != eventsPerClient || nB != eventsPerClient {
		t.Errorf("event counts: A=%d B=%d, want %d each", nA, nB, eventsPerClient)
	}
}
