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

package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

// writeMessageTimeout bounds how long [Codec.WriteMessage] will block waiting
// for a slow peer. WriteMessage satisfies [kvm.MessageWriter], whose signature
// has no context parameter, so the deadline is enforced internally.
const writeMessageTimeout = 5 * time.Second

// Codec adapts a single [websocket.Conn] to the read/write surface used by the
// rest of HydraKVM: text-frame JSON envelopes inbound and outbound, with
// WriteMessage satisfying [kvm.MessageWriter] for use by Application.
type Codec struct {
	conn *websocket.Conn
}

// NewCodec wraps conn for use by HydraKVM. The codec does not assume ownership
// of conn beyond what its own methods need; the caller is responsible for the
// surrounding HTTP request lifecycle.
func NewCodec(conn *websocket.Conn) *Codec { return &Codec{conn: conn} }

var _ kvm.MessageWriter = (*Codec)(nil)

// ReadFrame reads a single text frame and returns its raw payload bytes.
// Binary frames are rejected as a wire-protocol violation.
func (c *Codec) ReadFrame(ctx context.Context) ([]byte, error) {
	mt, data, err := c.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	if mt != websocket.MessageText {
		return nil, fmt.Errorf("websocket: expected text frame, got %v", mt)
	}
	return data, nil
}

// WriteFrame writes data as a single text frame.
func (c *Codec) WriteFrame(ctx context.Context, data []byte) error {
	return c.conn.Write(ctx, websocket.MessageText, data)
}

type outboundEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WriteMessage marshals payload to JSON, wraps it in a {type, payload}
// envelope, and writes it as a text frame. Because [kvm.MessageWriter] takes
// no context, the call is bounded by [writeMessageTimeout] internally.
func (c *Codec) WriteMessage(msgType string, payload any) error {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("websocket: marshal payload: %w", err)
		}
		raw = b
	}
	env, err := json.Marshal(outboundEnvelope{Type: msgType, Payload: raw})
	if err != nil {
		return fmt.Errorf("websocket: marshal envelope: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeMessageTimeout)
	defer cancel()
	return c.conn.Write(ctx, websocket.MessageText, env)
}

// Close closes the underlying connection with a normal-closure status and the
// supplied human-readable reason text.
func (c *Codec) Close(reason string) error {
	return c.conn.Close(websocket.StatusNormalClosure, reason)
}
