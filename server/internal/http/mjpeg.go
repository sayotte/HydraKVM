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
	nethttp "net/http"
	"sync"
	"sync/atomic"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

const mjpegBoundary = "hydrakvm-mjpeg"

// mjpegSink is the [kvm.FrameSink] consumed by the /stream/{token} handler.
// WriteFrame is non-blocking: if the buffered channel is full, the oldest
// queued frame is dropped to make room for the newest. Buffer of 2 keeps the
// stream "fresh" without growing unbounded if the HTTP writer stalls.
type mjpegSink struct {
	frames chan kvm.VideoFrame
	closed atomic.Bool
	mu     sync.Mutex
}

func newMJPEGSink() *mjpegSink {
	return &mjpegSink{frames: make(chan kvm.VideoFrame, 2)}
}

func (s *mjpegSink) WriteFrame(vf kvm.VideoFrame) {
	if s.closed.Load() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	for {
		select {
		case s.frames <- vf:
			return
		default:
			select {
			case <-s.frames:
			default:
			}
		}
	}
}

// Frames returns the receive channel for the streaming loop.
func (s *mjpegSink) Frames() <-chan kvm.VideoFrame { return s.frames }

// Close idempotently closes the underlying channel.
func (s *mjpegSink) Close() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.mu.Lock()
	close(s.frames)
	s.mu.Unlock()
}

func (s *Server) handleStream(w nethttp.ResponseWriter, r *nethttp.Request) {
	token := r.PathValue("token")
	if !validToken(token) {
		nethttp.NotFound(w, r)
		return
	}
	client, ok := s.streams.consume(token)
	if !ok {
		s.Logger.Warn("stream token rejected", "remote_addr", r.RemoteAddr)
		nethttp.NotFound(w, r)
		return
	}

	rc := nethttp.NewResponseController(w)
	sink := newMJPEGSink()
	client.SetVideoOut(sink)
	s.Logger.Info("stream connected", "remote_addr", r.RemoteAddr)

	defer func() {
		client.SetVideoOut(nil)
		sink.Close()
		s.Logger.Info("stream disconnected", "remote_addr", r.RemoteAddr)
	}()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+mjpegBoundary)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(nethttp.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	streamFrames(r.Context(), w, rc, sink.Frames())
}

func streamFrames(ctx context.Context, w nethttp.ResponseWriter, rc *nethttp.ResponseController, frames <-chan kvm.VideoFrame) {
	for {
		select {
		case <-ctx.Done():
			return
		case vf, ok := <-frames:
			if !ok {
				return
			}
			header := fmt.Sprintf("\r\n--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", mjpegBoundary, len(vf.Data))
			if _, err := w.Write([]byte(header)); err != nil {
				return
			}
			if _, err := w.Write(vf.Data); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
