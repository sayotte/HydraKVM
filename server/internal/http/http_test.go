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
	"errors"
	nethttp "net/http"
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
)

func TestNewServerStoresWiring(t *testing.T) {
	cfg := config.HTTPServerConfig{ListenAddr: ":0"}
	ap := &auth.NullProvider{}
	dr := dispatch.NewRouter()
	s := NewServer(cfg, ap, dr)
	if s.Config != cfg {
		t.Errorf("Config = %+v, want %+v", s.Config, cfg)
	}
	if s.Auth != ap {
		t.Errorf("Auth not stored")
	}
	if s.Dispatcher != dr {
		t.Errorf("Dispatcher not stored")
	}
}

func TestListenAndServeReturnsServerClosedOnShutdown(t *testing.T) {
	s := NewServer(config.HTTPServerConfig{ListenAddr: "127.0.0.1:0"}, &auth.NullProvider{}, dispatch.NewRouter())
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()

	// Give the listener a moment to bind. The race here is benign: even if
	// Shutdown beats ListenAndServe to the punch, ListenAndServe still
	// returns ErrServerClosed.
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
