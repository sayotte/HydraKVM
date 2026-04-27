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

import "context"

// Channel binds one VideoSource and one KeyEventSink to a per-Channel
// KeyboardState. Multiple Clients may drive the same Channel concurrently;
// writes are serialized through an unbuffered chan KeyEvent drained by
// [Channel.Run]. Backpressure flows upstream to whoever called
// [Channel.SendKeyEvent] (in production, the WebSocket reader goroutine via
// the dispatcher), which is the desired behavior — a wedged USB serial write
// should slow the offending Client, not silently grow a queue.
type Channel struct {
	VideoIn  VideoSource
	KeyOut   KeyEventSink
	KbdState KeyboardState

	keyCh chan KeyEvent
}

// NewChannel constructs a Channel. In production, [Application] launches
// [Channel.Run] when the first Client attaches and cancels its context when
// the last Client detaches; tests may launch Run directly.
func NewChannel(vs VideoSource, ks KeyEventSink) *Channel {
	return &Channel{
		VideoIn: vs,
		KeyOut:  ks,
		keyCh:   make(chan KeyEvent),
	}
}

// Run drains the Channel's key-event queue and dispatches each event to the
// KeyEventSink in arrival order. Returns when ctx is cancelled. There is
// exactly one Run goroutine per Channel; serialization is enforced by that
// single drainer plus the unbuffered queue on the producer side.
func (c *Channel) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ke := <-c.keyCh:
			if c.KeyOut != nil {
				c.KeyOut.ReportKeyEvent(ke)
			}
		}
	}
}

// SendKeyEvent enqueues a KeyEvent for serialized delivery to the Channel's
// KeyEventSink. Blocks until the drainer accepts it or ctx is cancelled.
func (c *Channel) SendKeyEvent(ctx context.Context, ke KeyEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.keyCh <- ke:
		return nil
	}
}
