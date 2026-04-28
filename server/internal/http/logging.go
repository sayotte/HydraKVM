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
	"log/slog"
	nethttp "net/http"
	"time"
)

// responseRecorder wraps an [nethttp.ResponseWriter] to capture status code
// and bytes written, for the access-log middleware.
type responseRecorder struct {
	nethttp.ResponseWriter
	status int
	bytes  int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if rr.status == 0 {
		rr.status = nethttp.StatusOK
	}
	n, err := rr.ResponseWriter.Write(b)
	rr.bytes += n
	return n, err
}

// Unwrap exposes the underlying ResponseWriter so [nethttp.ResponseController]
// (used by the WebSocket accept path for hijacking) can reach the real
// connection through this recorder wrapper.
func (rr *responseRecorder) Unwrap() nethttp.ResponseWriter {
	return rr.ResponseWriter
}

func accessLog(logger *slog.Logger, h nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w}
		h.ServeHTTP(rr, r)
		if rr.status == 0 {
			rr.status = nethttp.StatusOK
		}
		logger.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rr.status,
			"bytes", rr.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
