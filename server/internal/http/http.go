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
	nethttp "net/http"
	"time"

	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
)

// readHeaderTimeout caps how long the server will wait for a client to send
// the request headers. Picked conservatively to defeat Slowloris-style attacks
// without breaking legitimate slow networks.
const readHeaderTimeout = 10 * time.Second

// Server is the HydraKVM HTTP front end. Step 2 ships a stub: ListenAndServe
// blocks on the embedded [net/http.Server] but no handlers are wired yet.
type Server struct {
	netServer  *nethttp.Server
	Config     config.HTTPServerConfig
	Auth       auth.Provider
	Dispatcher *dispatch.Router
}

// NewServer constructs a Server from the given pieces. No socket is opened
// until [Server.ListenAndServe] is called.
func NewServer(cfg config.HTTPServerConfig, ap auth.Provider, dr *dispatch.Router) *Server {
	mux := nethttp.NewServeMux()
	return &Server{
		netServer: &nethttp.Server{
			Addr:              cfg.ListenAddr,
			Handler:           mux,
			ReadHeaderTimeout: readHeaderTimeout,
		},
		Config:     cfg,
		Auth:       ap,
		Dispatcher: dr,
	}
}

// ListenAndServe binds the configured listen address and serves until
// [Server.Shutdown] is called or the underlying listener fails.
func (s *Server) ListenAndServe() error {
	return s.netServer.ListenAndServe()
}

// Shutdown gracefully stops the server, draining in-flight requests until ctx
// is cancelled.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.netServer.Shutdown(ctx)
}
