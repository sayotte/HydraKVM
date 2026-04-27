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
	"testing"
)

type echoParams struct {
	Msg string `json:"msg"`
}

type echoResult struct {
	Echo string `json:"echo"`
}

func TestDispatchUnknownTypeReturnsErrNoHandler(t *testing.T) {
	r := NewRouter()
	_, err := r.Dispatch(t.Context(), Envelope{Type: "missing"})
	if !errors.Is(err, ErrNoHandler) {
		t.Errorf("got %v want ErrNoHandler", err)
	}
}

func TestRegisterRoundTripsTypedPayloadAndResult(t *testing.T) {
	r := NewRouter()
	Register(r, "echo", func(_ context.Context, p echoParams) (echoResult, error) {
		return echoResult{Echo: p.Msg}, nil
	})

	in := Envelope{
		Type:    "echo",
		ID:      "abc",
		Payload: json.RawMessage(`{"msg":"hello"}`),
	}
	reply, err := r.Dispatch(t.Context(), in)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if reply == nil {
		t.Fatal("expected non-nil reply")
	}
	if reply.Type != "echo" || reply.ID != "abc" {
		t.Errorf("reply Type/ID: got %q/%q want echo/abc", reply.Type, reply.ID)
	}
	var got echoResult
	if err := json.Unmarshal(reply.Payload, &got); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}
	if got.Echo != "hello" {
		t.Errorf("Echo: got %q want %q", got.Echo, "hello")
	}
}

func TestRegisterNotificationReturnsNilEnvelope(t *testing.T) {
	r := NewRouter()
	called := false
	RegisterNotification(r, "ping", func(_ context.Context, _ struct{}) error {
		called = true
		return nil
	})

	reply, err := r.Dispatch(t.Context(), Envelope{Type: "ping"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if reply != nil {
		t.Errorf("notification: got reply %+v want nil", reply)
	}
	if !called {
		t.Error("handler was not invoked")
	}
}

func TestRegisterPropagatesHandlerError(t *testing.T) {
	r := NewRouter()
	sentinel := errors.New("boom")
	Register(r, "fail", func(_ context.Context, _ struct{}) (struct{}, error) {
		return struct{}{}, sentinel
	})

	_, err := r.Dispatch(t.Context(), Envelope{Type: "fail"})
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v want sentinel", err)
	}
}

func TestRegisterReportsBadJSON(t *testing.T) {
	r := NewRouter()
	Register(r, "echo", func(_ context.Context, p echoParams) (echoResult, error) {
		return echoResult{Echo: p.Msg}, nil
	})

	_, err := r.Dispatch(t.Context(), Envelope{
		Type:    "echo",
		Payload: json.RawMessage(`not json`),
	})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}
