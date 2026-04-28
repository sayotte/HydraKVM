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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"

	"github.com/coder/websocket"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	"github.com/sayotte/hydrakvm/internal/http/web"
	wsockt "github.com/sayotte/hydrakvm/internal/http/websocket"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

func (s *Server) handleIndex(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.URL.Path != "/" {
		nethttp.NotFound(w, r)
		return
	}
	chans := s.App.Channels()
	tmplChans := make([]web.ChannelInfo, len(chans))
	for i, c := range chans {
		tmplChans[i] = web.ChannelInfo{ID: c.ID, Name: c.ID}
	}
	var buf bytes.Buffer
	if err := s.indexTmpl.Execute(&buf, web.IndexData{Title: "HydraKVM", Channels: tmplChans}); err != nil {
		s.Logger.Error("index render", "err", err)
		nethttp.Error(w, "template render failed", nethttp.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

type connectResponse struct {
	WSURL string `json:"ws_url"`
}

func (s *Server) handleAPIConnect(w nethttp.ResponseWriter, _ *nethttp.Request) {
	tok, err := s.tokens.mint()
	if err != nil {
		s.Logger.Error("token mint", "err", err)
		nethttp.Error(w, "internal error", nethttp.StatusInternalServerError)
		return
	}
	s.Logger.Debug("token minted")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(connectResponse{WSURL: "/ws/" + tok})
}

func (s *Server) handleWS(w nethttp.ResponseWriter, r *nethttp.Request) {
	token := r.PathValue("token")
	if !validToken(token) || !s.tokens.consume(token) {
		s.Logger.Warn("token rejected", "remote_addr", r.RemoteAddr)
		nethttp.NotFound(w, r)
		return
	}
	s.Logger.Debug("token consumed")

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.Logger.Warn("ws accept", "err", err, "remote_addr", r.RemoteAddr)
		return
	}
	s.Logger.Info("ws connected", "remote_addr", r.RemoteAddr)

	codec := wsockt.NewCodec(conn)
	client := &kvm.Client{Outbound: codec}
	s.App.AddClient(client)

	defer func() {
		s.App.RemoveClient(client)
		_ = codec.Close("server done")
		s.Logger.Info("ws disconnected", "remote_addr", r.RemoteAddr)
	}()

	s.runDispatchLoop(r.Context(), codec, client)
}

func (s *Server) runDispatchLoop(ctx context.Context, codec *wsockt.Codec, client *kvm.Client) {
	ctx = kvm.WithClient(ctx, client)
	for {
		data, err := codec.ReadFrame(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
				s.Logger.Info("ws read closed", "status", status.String())
			} else if !errors.Is(err, context.Canceled) {
				s.Logger.Warn("ws read", "err", err)
			}
			return
		}
		var env dispatch.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			s.Logger.Warn("ws bad envelope", "err", err)
			continue
		}
		reply, err := s.Dispatcher.Dispatch(ctx, env)
		if err != nil {
			s.Logger.Warn("ws dispatch", "type", env.Type, "err", err)
			continue
		}
		if reply == nil {
			continue
		}
		b, err := json.Marshal(reply)
		if err != nil {
			s.Logger.Error("ws marshal reply", "err", err)
			continue
		}
		if err := codec.WriteFrame(ctx, b); err != nil {
			s.Logger.Warn("ws write", "err", err)
			return
		}
	}
}
