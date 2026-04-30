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
)

// Channel binds one VideoSource and one KeyEventSink to a per-Channel
// KeyboardState. Multiple Clients may drive the same Channel concurrently;
// edges are serialized through an unbuffered chan drained by [Channel.Run],
// which applies each edge to KbdState and emits a fully-stamped KeyEvent to
// KeyOut. Backpressure flows upstream to whoever called
// [Channel.SendKeyEdge] (in production, the WebSocket reader goroutine via
// the dispatcher), which is the desired behavior — a wedged USB serial write
// should slow the offending Client, not silently grow a queue.
//
// Channel.Run also fans out frames from VideoIn to every attached Client's
// VideoOut FrameSink. WriteFrame calls are non-blocking by FrameSink contract:
// a slow client drops frames, never blocks other clients or the channel pump.
type Channel struct {
	VideoIn  VideoSource
	KeyOut   KeyEventSink
	KbdState KeyboardState

	edgeCh chan keyEdge

	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// keyEdge is the producer-side payload on the Channel's edge queue. Modifier
// state is resolved on the consumer side (the Run drainer), so the chan order
// is the sole linearizer for both KbdState mutation and sink delivery.
type keyEdge struct {
	Code KeyCode
	Type KeyType
}

// NewChannel constructs a Channel. In production, [Application] launches
// [Channel.Run] when the first Client attaches and cancels its context when
// the last Client detaches; tests may launch Run directly.
func NewChannel(vs VideoSource, ks KeyEventSink) *Channel {
	return &Channel{
		VideoIn: vs,
		KeyOut:  ks,
		edgeCh:  make(chan keyEdge),
		clients: make(map[*Client]struct{}),
	}
}

// RegisterClient adds c to the set receiving fan-out video frames while this
// Channel is running. Idempotent.
func (c *Channel) RegisterClient(cl *Client) {
	c.mu.Lock()
	c.clients[cl] = struct{}{}
	c.mu.Unlock()
}

// UnregisterClient removes c from the fan-out set. Idempotent.
func (c *Channel) UnregisterClient(cl *Client) {
	c.mu.Lock()
	delete(c.clients, cl)
	c.mu.Unlock()
}

// Run drains the Channel's edge queue, mutates KbdState in arrival order, and
// dispatches each fully-stamped KeyEvent to the KeyEventSink; in parallel it
// pumps frames from VideoIn to every registered Client. Returns when ctx is
// cancelled. There is exactly one Run goroutine per Channel; the single
// drainer is the sole writer to KbdState, so no further synchronization is
// needed.
func (c *Channel) Run(ctx context.Context) {
	var wg sync.WaitGroup
	if c.VideoIn != nil {
		wg.Go(func() { c.runVideoPump(ctx) })
	}
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case e := <-c.edgeCh:
			if e.Code.IsModifier() {
				bit := e.Code.ModifierBit()
				if e.Type == KeyTypeDown {
					c.KbdState.Modifiers |= bit
				} else {
					c.KbdState.Modifiers &^= bit
				}
			}
			if c.KeyOut != nil {
				c.KeyOut.ReportKeyEvent(KeyEvent{
					Code:      e.Code,
					Type:      e.Type,
					Modifiers: c.KbdState.Modifiers,
				})
			}
		}
	}
}

func (c *Channel) runVideoPump(ctx context.Context) {
	frames := c.VideoIn.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case vf, ok := <-frames:
			if !ok {
				return
			}
			c.fanout(vf)
		}
	}
}

func (c *Channel) fanout(vf VideoFrame) {
	c.mu.RLock()
	for cl := range c.clients {
		if sink := cl.VideoOut(); sink != nil {
			sink.WriteFrame(vf)
		}
	}
	c.mu.RUnlock()
}

// SendKeyEdge enqueues a key edge for serialized delivery. The Channel's Run
// goroutine resolves the post-edge modifier mask and emits the stamped
// KeyEvent to KeyOut in chan-arrival order. Blocks until the drainer accepts
// the edge or ctx is cancelled.
func (c *Channel) SendKeyEdge(ctx context.Context, code KeyCode, typ KeyType) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.edgeCh <- keyEdge{Code: code, Type: typ}:
		return nil
	}
}
