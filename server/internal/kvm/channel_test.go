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
	"sync"
	"testing"
	"time"
)

// recordingSink records events in arrival order under a mutex; it also
// asserts at most one concurrent ReportKeyEvent call by tracking an in-flight
// counter.
type recordingSink struct {
	mu       sync.Mutex
	events   []KeyEvent
	inFlight int
	maxSeen  int
	delay    time.Duration
}

func (s *recordingSink) ReportKeyEvent(ke KeyEvent) {
	s.mu.Lock()
	s.inFlight++
	if s.inFlight > s.maxSeen {
		s.maxSeen = s.inFlight
	}
	s.mu.Unlock()

	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	s.mu.Lock()
	s.events = append(s.events, ke)
	s.inFlight--
	s.mu.Unlock()
}

func TestChannelRunDeliversInOrder(t *testing.T) {
	sink := &recordingSink{}
	ch := NewChannel(nil, sink)

	ctx := t.Context()

	go ch.Run(ctx)

	want := []KeyEvent{
		{Code: KeyA, Type: KeyTypeDown},
		{Code: KeyA, Type: KeyTypeUp},
		{Code: KeyB, Type: KeyTypeDown},
	}
	for _, ke := range want {
		if err := ch.SendKeyEvent(ctx, ke); err != nil {
			t.Fatalf("SendKeyEvent: %v", err)
		}
	}

	// Drain: send a sentinel to ensure prior events were processed.
	if err := ch.SendKeyEvent(ctx, KeyEvent{Code: KeyZ, Type: KeyTypeDown}); err != nil {
		t.Fatalf("sentinel SendKeyEvent: %v", err)
	}
	// After SendKeyEvent for the sentinel returns, the drainer has read it
	// from the unbuffered channel; the previous event has already entered
	// ReportKeyEvent. We need one more synchronization — wait until events
	// length reaches 4.
	deadline := time.Now().Add(time.Second)
	for {
		sink.mu.Lock()
		n := len(sink.events)
		sink.mu.Unlock()
		if n == len(want)+1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for sink to receive %d events; got %d", len(want)+1, n)
		}
		time.Sleep(time.Millisecond)
	}

	for i, ke := range want {
		if sink.events[i] != ke {
			t.Errorf("event %d: got %+v want %+v", i, sink.events[i], ke)
		}
	}
}

func TestChannelSendKeyEventRespectsContextCancel(t *testing.T) {
	ch := NewChannel(nil, &recordingSink{})
	// No Run goroutine — every send will block.

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ch.SendKeyEvent(ctx, KeyEvent{Code: KeyA, Type: KeyTypeDown})
	if err == nil {
		t.Fatal("expected error when context is cancelled before send")
	}
}
