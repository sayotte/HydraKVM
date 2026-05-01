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
	"sync/atomic"
)

// Client is one connected user (one browser tab). It is a passive bag of
// sinks; the Client-to-Channel association is owned by Application, not by
// the Client itself.
//
// The video sink is installed/cleared asynchronously by the HTTP layer when
// an MJPEG connection arrives or drops; reads (Channel video pump) are
// coordinated via [atomic.Pointer].
type Client struct {
	videoOut atomic.Pointer[FrameSink]
	Outbound MessageWriter

	// MJPEGStreamURL is the per-Client MJPEG endpoint URL minted by the HTTP
	// layer at connect time. Application re-emits it as an [MsgMJPEGURL]
	// hint alongside [MsgClientUpdate] on video failover and recovery so the
	// browser can re-attach its long-lived <img> source.
	MJPEGStreamURL string
}

// SetVideoOut installs s as the Client's frame sink (or clears it when s is
// nil). Safe for concurrent use with the Channel video pump.
func (c *Client) SetVideoOut(s FrameSink) {
	if s == nil {
		c.videoOut.Store(nil)
		return
	}
	c.videoOut.Store(&s)
}

// VideoOut returns the currently installed frame sink, or nil if none.
func (c *Client) VideoOut() FrameSink {
	if p := c.videoOut.Load(); p != nil {
		return *p
	}
	return nil
}

// ClearVideoOutIf atomically clears the sink only if it still equals s.
// Returns true if the clear happened. The HTTP layer uses this so a
// connection that re-fetches /stream/{token} (overlapping with the
// previous connection during channel switch / failover) cannot have its
// freshly-installed sink nil'd by the older connection's teardown.
func (c *Client) ClearVideoOutIf(s FrameSink) bool {
	p := c.videoOut.Load()
	if p == nil || *p != s {
		return false
	}
	return c.videoOut.CompareAndSwap(p, nil)
}

// ctxKey is an unexported type to avoid collisions with other packages'
// context keys.
type ctxKey int

const clientCtxKey ctxKey = 1

// WithClient returns a derived context that carries the supplied Client.
// Dispatch handlers registered against [Application] methods retrieve the
// caller via [ClientFromContext].
func WithClient(ctx context.Context, c *Client) context.Context {
	return context.WithValue(ctx, clientCtxKey, c)
}

// ClientFromContext returns the Client stored in ctx by [WithClient], or nil
// if none is present.
func ClientFromContext(ctx context.Context) *Client {
	c, _ := ctx.Value(clientCtxKey).(*Client)
	return c
}
