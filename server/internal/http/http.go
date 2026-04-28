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
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	nethttp "net/http"
	"os"
	"time"

	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	"github.com/sayotte/hydrakvm/internal/http/web"
	"github.com/sayotte/hydrakvm/internal/kvm"
)

// readHeaderTimeout caps how long the server will wait for a client to send
// the request headers. Picked conservatively to defeat Slowloris-style attacks
// without breaking legitimate slow networks.
const readHeaderTimeout = 10 * time.Second

// Server is the HydraKVM HTTP front end. It owns the listener, the parsed
// HTML index template, the embedded static asset filesystem, and the
// pending-WebSocket-token registry.
type Server struct {
	netServer *nethttp.Server

	Config     config.HTTPServerConfig
	Auth       auth.Provider
	Dispatcher *dispatch.Router
	App        *kvm.Application
	Logger     *slog.Logger

	indexTmpl *template.Template
	staticFS  fs.FS
	tokens    *tokenRegistry
	streams   *streamRegistry
}

// NewServer constructs a Server. Parses the index template eagerly; an
// invalid template is a wiring bug and fails fast. If logger is nil, a
// JSON logger writing to stderr at INFO is installed.
func NewServer(
	cfg config.HTTPServerConfig,
	ap auth.Provider,
	dr *dispatch.Router,
	app *kvm.Application,
	logger *slog.Logger,
) (*Server, error) {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	tmpl, err := web.IndexTemplate()
	if err != nil {
		return nil, fmt.Errorf("http: parse index template: %w", err)
	}
	sfs, err := web.StaticFS()
	if err != nil {
		return nil, fmt.Errorf("http: open static fs: %w", err)
	}

	s := &Server{
		Config:     cfg,
		Auth:       ap,
		Dispatcher: dr,
		App:        app,
		Logger:     logger,
		indexTmpl:  tmpl,
		staticFS:   sfs,
		tokens:     newTokenRegistry(),
		streams:    newStreamRegistry(),
	}

	mux := nethttp.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /api/connect", s.handleAPIConnect)
	mux.HandleFunc("GET /ws/{token}", s.handleWS)
	mux.HandleFunc("GET /stream/{token}", s.handleStream)
	mux.Handle("GET /static/", nethttp.StripPrefix("/static/", nethttp.FileServer(nethttp.FS(sfs))))

	s.netServer = &nethttp.Server{
		Addr:              cfg.ListenAddr,
		Handler:           accessLog(logger, mux),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	return s, nil
}

// ListenAndServe binds the configured listen address and serves until
// [Server.Shutdown] is called or the underlying listener fails.
func (s *Server) ListenAndServe() error {
	s.Logger.Info("listening", "addr", s.Config.ListenAddr)
	return s.netServer.ListenAndServe()
}

// Shutdown gracefully stops the server, draining in-flight requests until ctx
// is cancelled.
func (s *Server) Shutdown(ctx context.Context) error {
	s.Logger.Info("shutting down")
	err := s.netServer.Shutdown(ctx)
	s.Logger.Info("shutdown complete")
	return err
}
