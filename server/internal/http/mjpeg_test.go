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
	"bufio"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

// tickingJPEGSource is a minimal kvm.VideoSource for testing the MJPEG
// pipeline without importing the synthetic package (which would invert the
// declared http -> kvm dependency direction).
type tickingJPEGSource struct{}

// minimalJPEG is a 1x1 black JPEG produced offline via image/jpeg; embedded as
// a byte slice so the test does not need to encode each iteration.
var minimalJPEG = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00,
	0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
}

func (tickingJPEGSource) Shape() kvm.StreamShape { return kvm.StreamShape{Codec: "mjpeg"} }
func (tickingJPEGSource) InitData() []byte       { return nil }
func (tickingJPEGSource) RequestKeyframe() error { return nil }
func (tickingJPEGSource) Subscribe(ctx context.Context) <-chan kvm.VideoFrame {
	ch := make(chan kvm.VideoFrame, 1)
	go func() {
		defer close(ch)
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				select {
				case ch <- kvm.VideoFrame{Data: minimalJPEG, IsKey: true}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

func newStreamingTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	app := kvm.NewApplication(t.Context())
	defaultCh := kvm.NewChannel(tickingJPEGSource{}, nil)
	app.AddChannel("__default__", defaultCh)
	app.DefaultChannel = defaultCh

	router := dispatch.NewRouter()
	dispatch.Register(router, kvm.MsgSwitchChannel, app.SwitchChannel)
	s, err := NewServer(config.HTTPServerConfig{ListenAddr: "127.0.0.1:0"}, &auth.NullProvider{}, router, app, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.netServer.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestStreamBadTokenIs404(t *testing.T) {
	ts := newStreamingTestServer(t)
	resp, err := ts.Client().Get(ts.URL + "/stream/badtoken")
	if err != nil {
		t.Fatalf("GET /stream/bad: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != nethttp.StatusNotFound {
		t.Errorf("status = %d want 404", resp.StatusCode)
	}
}

func TestStreamServesMultipartJPEG(t *testing.T) {
	ts := newStreamingTestServer(t)

	connResp, err := ts.Client().Get(ts.URL + "/api/connect")
	if err != nil {
		t.Fatalf("GET /api/connect: %v", err)
	}
	var cr connectResponse
	if err := json.NewDecoder(connResp.Body).Decode(&cr); err != nil {
		t.Fatalf("decode connect: %v", err)
	}
	_ = connResp.Body.Close()

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

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read mjpeg url: %v", err)
	}
	var env struct {
		Type    string             `json:"type"`
		Payload kvm.MJPEGURLParams `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal mjpeg envelope: %v", err)
	}
	if env.Type != kvm.MsgMJPEGURL {
		t.Fatalf("envelope type = %q want %q", env.Type, kvm.MsgMJPEGURL)
	}
	if !strings.HasPrefix(env.Payload.URL, "/stream/") {
		t.Fatalf("mjpeg url = %q does not start with /stream/", env.Payload.URL)
	}

	streamCtx, streamCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer streamCancel()
	req, err := nethttp.NewRequestWithContext(streamCtx, nethttp.MethodGet, ts.URL+env.Payload.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := ts.Client().Do(req) //nolint:gosec // test-only; URL is built from httptest server
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != nethttp.StatusOK {
		t.Fatalf("stream status = %d want 200", resp.StatusCode)
	}

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	if mediaType != "multipart/x-mixed-replace" {
		t.Errorf("media type = %q want multipart/x-mixed-replace", mediaType)
	}
	boundary, ok := params["boundary"]
	if !ok || boundary != mjpegBoundary {
		t.Errorf("boundary = %q want %q", boundary, mjpegBoundary)
	}

	mr := multipart.NewReader(bufio.NewReader(resp.Body), boundary)
	for i := range 2 {
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		ct := part.Header.Get("Content-Type")
		if ct != "image/jpeg" {
			t.Errorf("part %d content-type = %q want image/jpeg", i, ct)
		}
		body, err := io.ReadAll(part)
		_ = part.Close()
		if err != nil {
			t.Fatalf("part %d read: %v", i, err)
		}
		if len(body) < 3 || body[0] != 0xFF || body[1] != 0xD8 || body[2] != 0xFF {
			t.Errorf("part %d not JPEG SOI", i)
		}
	}
}
