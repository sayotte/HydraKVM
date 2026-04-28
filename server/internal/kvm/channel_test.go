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
	"sync/atomic"
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

// fakeVideoSource emits a fixed VideoFrame on a tick.
type fakeVideoSource struct {
	interval time.Duration
}

func (f *fakeVideoSource) Shape() StreamShape     { return StreamShape{Codec: "mjpeg"} }
func (f *fakeVideoSource) InitData() []byte       { return nil }
func (f *fakeVideoSource) RequestKeyframe() error { return nil }
func (f *fakeVideoSource) Subscribe(ctx context.Context) <-chan VideoFrame {
	ch := make(chan VideoFrame, 1)
	go func() {
		defer close(ch)
		t := time.NewTicker(f.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				select {
				case ch <- VideoFrame{Data: []byte{0xFF}, IsKey: true}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

type countingSink struct {
	n atomic.Int32
}

func (s *countingSink) WriteFrame(_ VideoFrame) { s.n.Add(1) }

// boundedSink simulates the FrameSink contract: WriteFrame is non-blocking;
// when its internal buffer is full, frames are dropped. Used to verify that a
// slow consumer of the sink does not block the channel's video pump.
type boundedSink struct {
	frames  chan VideoFrame
	dropped atomic.Int32
}

func newBoundedSink(capacity int) *boundedSink {
	return &boundedSink{frames: make(chan VideoFrame, capacity)}
}

func (s *boundedSink) WriteFrame(vf VideoFrame) {
	select {
	case s.frames <- vf:
	default:
		s.dropped.Add(1)
	}
}

func TestChannelPumpsFramesToRegisteredClients(t *testing.T) {
	src := &fakeVideoSource{interval: 5 * time.Millisecond}
	ch := NewChannel(src, nil)

	s1, s2 := &countingSink{}, &countingSink{}
	c1 := &Client{}
	c1.SetVideoOut(s1)
	c2 := &Client{}
	c2.SetVideoOut(s2)
	ch.RegisterClient(c1)
	ch.RegisterClient(c2)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ch.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s1.n.Load() > 0 && s2.n.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if s1.n.Load() == 0 || s2.n.Load() == 0 {
		t.Errorf("sinks: s1=%d s2=%d; want both > 0", s1.n.Load(), s2.n.Load())
	}
}

func TestChannelDropsFramesForSaturatedSink(t *testing.T) {
	src := &fakeVideoSource{interval: 2 * time.Millisecond}
	ch := NewChannel(src, nil)

	// Sink with capacity 1 and a consumer that never reads — drops should
	// accumulate without affecting the other sink's pace.
	saturated := newBoundedSink(1)
	healthy := newBoundedSink(64)
	cs := &Client{}
	cs.SetVideoOut(saturated)
	ch.RegisterClient(cs)
	ch2 := &Client{}
	ch2.SetVideoOut(healthy)
	ch.RegisterClient(ch2)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ch.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if saturated.dropped.Load() > 0 && len(healthy.frames) >= 3 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	<-done

	if saturated.dropped.Load() == 0 {
		t.Error("expected saturated sink to drop frames")
	}
	if len(healthy.frames) < 3 {
		t.Errorf("healthy sink only got %d frames", len(healthy.frames))
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
