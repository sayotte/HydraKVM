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

package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrNoHandler is returned by [Router.Dispatch] when no handler is
// registered for the inbound Envelope's Type.
var ErrNoHandler = errors.New("no handler registered for message type")

// Envelope is the protocol-agnostic shape of every inbound (and reply)
// dispatch message. Type names a registered handler; ID is opaque correlation
// data echoed onto any reply Envelope; Payload is the (possibly nil) handler
// parameters as raw JSON.
type Envelope struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Handler is the low-level handler shape: receives a context plus raw JSON
// payload and returns a value to be JSON-encoded into the reply Envelope (or
// nil for notification-style messages).
type Handler func(ctx context.Context, payload json.RawMessage) (any, error)

// Router holds the registered Handlers and dispatches Envelopes to them.
// Concurrent calls to Dispatch are safe.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRouter returns an empty Router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]Handler)}
}

// Handle registers h for msgType. A second call for the same type
// overwrites; this is intentional (the wiring layer in cmd/hydrakvm is the
// only registrar).
func (r *Router) Handle(msgType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[msgType] = h
}

// Dispatch looks up the Handler for env.Type and invokes it. The returned
// Envelope (if non-nil) carries the same Type and ID as env, with Payload
// set to the JSON-encoded handler result. If the handler returns a nil
// result (notification-style), Dispatch returns (nil, nil) on success.
func (r *Router) Dispatch(ctx context.Context, env Envelope) (*Envelope, error) {
	r.mu.RLock()
	h, ok := r.handlers[env.Type]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoHandler, env.Type)
	}
	res, err := h(ctx, env.Payload)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("marshal reply for %q: %w", env.Type, err)
	}
	return &Envelope{Type: env.Type, ID: env.ID, Payload: payload}, nil
}

// Register attaches a typed request/response handler. P and R are the
// concrete parameter and result types; payload JSON is unmarshalled into a
// fresh P before fn is called, and fn's R return is JSON-encoded into the
// reply Envelope by [Router.Dispatch].
func Register[P, R any](r *Router, msgType string, fn func(context.Context, P) (R, error)) {
	r.Handle(msgType, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var p P
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &p); err != nil {
				return nil, fmt.Errorf("unmarshal %q payload: %w", msgType, err)
			}
		}
		return fn(ctx, p)
	})
}

// RegisterNotification attaches a typed notification handler — one that has
// no reply. The handler's error is propagated; on success Dispatch returns
// (nil, nil).
func RegisterNotification[P any](r *Router, msgType string, fn func(context.Context, P) error) {
	r.Handle(msgType, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var p P
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &p); err != nil {
				return nil, fmt.Errorf("unmarshal %q payload: %w", msgType, err)
			}
		}
		return nil, fn(ctx, p)
	})
}
