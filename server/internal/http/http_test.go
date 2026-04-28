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

package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	app := kvm.NewApplication(t.Context())
	app.AddChannel("ch1", kvm.NewChannel(nil, nil))
	router := dispatch.NewRouter()
	dispatch.Register(router, kvm.MsgSwitchChannel, app.SwitchChannel)
	s, err := NewServer(config.HTTPServerConfig{ListenAddr: "127.0.0.1:0"}, &auth.NullProvider{}, router, app, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.netServer.Handler)
	t.Cleanup(ts.Close)
	return s, ts
}

func TestNewServerStoresWiring(t *testing.T) {
	cfg := config.HTTPServerConfig{ListenAddr: ":0"}
	ap := &auth.NullProvider{}
	dr := dispatch.NewRouter()
	app := kvm.NewApplication(t.Context())
	s, err := NewServer(cfg, ap, dr, app, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if s.Config != cfg {
		t.Errorf("Config = %+v, want %+v", s.Config, cfg)
	}
	if s.Auth != ap {
		t.Errorf("Auth not stored")
	}
	if s.Dispatcher != dr {
		t.Errorf("Dispatcher not stored")
	}
	if s.App != app {
		t.Errorf("App not stored")
	}
}

func TestListenAndServeReturnsServerClosedOnShutdown(t *testing.T) {
	app := kvm.NewApplication(t.Context())
	s, err := NewServer(config.HTTPServerConfig{ListenAddr: "127.0.0.1:0"}, &auth.NullProvider{}, dispatch.NewRouter(), app, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-errCh:
		if !errors.Is(err, nethttp.ErrServerClosed) {
			t.Errorf("ListenAndServe returned %v, want ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ListenAndServe did not return after Shutdown")
	}
}

func TestIndexRendersChannelOptions(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != nethttp.StatusOK {
		t.Errorf("status = %d want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q want text/html...", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, `id="channel-selector"`) {
		t.Errorf("missing channel-selector; body:\n%s", s)
	}
	if !strings.Contains(s, `<option value="ch1">`) {
		t.Errorf("missing ch1 option; body:\n%s", s)
	}
}

func TestAPIConnectReturnsTokenURL(t *testing.T) {
	_, ts := newTestServer(t)
	urlRe := regexp.MustCompile(`^/ws/[A-Za-z0-9_-]+$`)

	tokens := map[string]bool{}
	for range 2 {
		resp, err := ts.Client().Get(ts.URL + "/api/connect")
		if err != nil {
			t.Fatalf("GET /api/connect: %v", err)
		}
		var cr connectResponse
		if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = resp.Body.Close()
		if cr.WSURL == "" || !urlRe.MatchString(cr.WSURL) {
			t.Errorf("ws_url = %q does not match %s", cr.WSURL, urlRe)
		}
		if tokens[cr.WSURL] {
			t.Errorf("duplicate ws_url across mints: %q", cr.WSURL)
		}
		tokens[cr.WSURL] = true
	}
}

func TestWSBadTokenRejected(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := ts.Client().Get(ts.URL + "/ws/notarealtoken")
	if err != nil {
		t.Fatalf("GET /ws/bad: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != nethttp.StatusNotFound {
		t.Errorf("status = %d want 404", resp.StatusCode)
	}
}

func TestWSRoundTripSwitchChannel(t *testing.T) {
	_, ts := newTestServer(t)

	resp, err := ts.Client().Get(ts.URL + "/api/connect")
	if err != nil {
		t.Fatalf("GET /api/connect: %v", err)
	}
	var cr connectResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	_ = resp.Body.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + cr.WSURL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	if dialResp != nil && dialResp.Body != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "test done") }()

	env := dispatch.Envelope{Type: kvm.MsgSwitchChannel, ID: "test-1", Payload: json.RawMessage(`{"channel_id":"ch1"}`)}
	b, _ := json.Marshal(env)
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var reply dispatch.Envelope
	if err := json.Unmarshal(data, &reply); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}
	if reply.Type != kvm.MsgSwitchChannel || reply.ID != "test-1" {
		t.Errorf("reply envelope = %+v", reply)
	}
	var res kvm.SwitchChannelResult
	if err := json.Unmarshal(reply.Payload, &res); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if res.ChannelID != "ch1" {
		t.Errorf("result channel_id = %q want ch1", res.ChannelID)
	}
}
