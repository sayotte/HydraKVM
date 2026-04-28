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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	cws "github.com/coder/websocket"
)

// serverPair stands up a test WS server whose handler hands the accepted Conn
// to fn (running on a server-side goroutine) and waits for fn to return before
// finalizing the response. The returned client conn is wired to that server.
func serverPair(t *testing.T, fn func(*Codec)) (*cws.Conn, func()) {
	t.Helper()
	var wg sync.WaitGroup
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := cws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("server accept: %v", err)
			return
		}
		wg.Add(1)
		defer wg.Done()
		fn(NewCodec(conn))
	}))
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, resp, err := cws.Dial(ctx, url, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("client dial: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	cleanup := func() {
		_ = client.Close(cws.StatusNormalClosure, "test done")
		wg.Wait()
		srv.Close()
	}
	return client, cleanup
}

func TestCodecReadFrame(t *testing.T) {
	got := make(chan []byte, 1)
	client, cleanup := serverPair(t, func(c *Codec) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := c.ReadFrame(ctx)
		if err != nil {
			t.Errorf("server ReadFrame: %v", err)
			return
		}
		got <- data
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Write(ctx, cws.MessageText, []byte("hello")); err != nil {
		t.Fatalf("client Write: %v", err)
	}
	select {
	case data := <-got:
		if string(data) != "hello" {
			t.Fatalf("ReadFrame = %q, want %q", data, "hello")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ReadFrame result")
	}
}

func TestCodecWriteFrame(t *testing.T) {
	client, cleanup := serverPair(t, func(c *Codec) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.WriteFrame(ctx, []byte("world")); err != nil {
			t.Errorf("server WriteFrame: %v", err)
		}
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mt, data, err := client.Read(ctx)
	if err != nil {
		t.Fatalf("client Read: %v", err)
	}
	if mt != cws.MessageText {
		t.Fatalf("client got message type %v, want text", mt)
	}
	if string(data) != "world" {
		t.Fatalf("client got %q, want %q", data, "world")
	}
}

func TestCodecWriteMessage(t *testing.T) {
	client, cleanup := serverPair(t, func(c *Codec) {
		if err := c.WriteMessage("ping", map[string]string{"hello": "there"}); err != nil {
			t.Errorf("server WriteMessage: %v", err)
		}
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mt, data, err := client.Read(ctx)
	if err != nil {
		t.Fatalf("client Read: %v", err)
	}
	if mt != cws.MessageText {
		t.Fatalf("got message type %v, want text", mt)
	}
	var env struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v (raw=%s)", err, data)
	}
	if env.Type != "ping" {
		t.Fatalf("envelope type = %q, want %q", env.Type, "ping")
	}
	if env.Payload["hello"] != "there" {
		t.Fatalf("envelope payload = %v, want hello=there", env.Payload)
	}
}

func TestCodecReadFrameRejectsBinary(t *testing.T) {
	gotErr := make(chan error, 1)
	client, cleanup := serverPair(t, func(c *Codec) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c.ReadFrame(ctx)
		gotErr <- err
	})
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Write(ctx, cws.MessageBinary, []byte{0x01, 0x02}); err != nil {
		t.Fatalf("client Write: %v", err)
	}
	select {
	case err := <-gotErr:
		if err == nil {
			t.Fatal("ReadFrame returned nil error on binary frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ReadFrame error")
	}
}

func TestCodecClose(t *testing.T) {
	closed := make(chan error, 1)
	client, cleanup := serverPair(t, func(c *Codec) {
		closed <- c.Close("bye")
	})
	defer cleanup()

	readErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err := client.Read(ctx)
		readErr <- err
	}()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for server-side Close")
	}
	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("client Read after server close: want error, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client Read to error")
	}
}
