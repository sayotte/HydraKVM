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

package v4l

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

// captureHandler is a [slog.Handler] that records emitted messages and
// attributes for tests that need to assert on log output.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *captureHandler) findErr(substr string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		var errStr string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "err" {
				if err, ok := a.Value.Any().(error); ok {
					errStr = err.Error()
				} else {
					errStr = a.Value.String()
				}
				return false
			}
			return true
		})
		if errStr != "" && strings.Contains(errStr, substr) {
			return true
		}
	}
	return false
}

// TestMain implements the helper-process pattern from os/exec's stdlib
// tests: when invoked with a special env var the test binary impersonates
// ffmpeg by emitting synthetic JPEGs to stdout and (optionally) hanging or
// exiting on cue. Otherwise it runs tests normally.
func TestMain(m *testing.M) {
	switch os.Getenv("V4L_TEST_HELPER") {
	case "frames":
		runHelperFrames()
	case "exit-immediately":
		os.Exit(0)
	case "hang-no-frames":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "one-frame-then-hang":
		_, _ = os.Stdout.Write(fakeJPEG(0xA0, 16))
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "", "off":
		os.Exit(m.Run())
	default:
		fmt.Fprintf(os.Stderr, "unknown V4L_TEST_HELPER=%q\n", os.Getenv("V4L_TEST_HELPER"))
		os.Exit(2)
	}
}

func runHelperFrames() {
	count, _ := strconv.Atoi(os.Getenv("V4L_TEST_HELPER_FRAMES"))
	if count <= 0 {
		count = 1 << 30
	}
	intervalMS, _ := strconv.Atoi(os.Getenv("V4L_TEST_HELPER_INTERVAL_MS"))
	if intervalMS <= 0 {
		intervalMS = 10
	}
	for i := range count {
		marker := byte(0xA0 + (i % 0x40))
		if _, err := os.Stdout.Write(fakeJPEG(marker, 16)); err != nil {
			os.Exit(0)
		}
		time.Sleep(time.Duration(intervalMS) * time.Millisecond)
	}
	os.Exit(0)
}

// helperCommand returns a commandFunc that re-execs the test binary in
// helper mode with the given env var values.
func helperCommand(t *testing.T, mode string, extraEnv ...string) commandFunc {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return func(ctx context.Context, _ Config) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe)
		env := append(os.Environ(), "V4L_TEST_HELPER="+mode)
		env = append(env, extraEnv...)
		cmd.Env = env
		return cmd
	}
}

func newTestStream(t *testing.T, mode string, cfg Config, extraEnv ...string) *MJPEGStream {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	if cfg.RetryInterval == 0 {
		cfg.RetryInterval = 50 * time.Millisecond
	}
	if cfg.FrameTimeout == 0 {
		cfg.FrameTimeout = 200 * time.Millisecond
	}
	if cfg.FirstFrameTimeout == 0 {
		cfg.FirstFrameTimeout = 500 * time.Millisecond
	}
	s := New(cfg, logger)
	s.command = helperCommand(t, mode, extraEnv...)
	return s
}

func TestStreamDeliversFrames(t *testing.T) {
	s := newTestStream(t, "frames", Config{DevicePath: "/dev/null"})
	defer s.Close()

	ctx := t.Context()
	ch := s.Subscribe(ctx)

	for i := range 3 {
		select {
		case vf, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed before frame %d", i)
			}
			if len(vf.Data) < 4 || vf.Data[0] != 0xFF || vf.Data[1] != 0xD8 {
				t.Errorf("frame %d not a JPEG: %x", i, vf.Data[:min(8, len(vf.Data))])
			}
			if !vf.IsKey {
				t.Errorf("frame %d: IsKey=false", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no frame %d within 2s", i)
		}
	}
}

func TestStreamCloseStopsSupervisor(t *testing.T) {
	s := newTestStream(t, "frames", Config{DevicePath: "/dev/null"})
	ctx := t.Context()
	ch := s.Subscribe(ctx)

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no first frame")
	}

	done := make(chan struct{})
	go func() {
		s.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return")
	}

	// After Close, the channel should be drained/closed.
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel did not close after Close")
		}
	}
}

func TestStreamSubprocessExitClosesSubscribers(t *testing.T) {
	s := newTestStream(t, "exit-immediately", Config{
		DevicePath:    "/dev/null",
		RetryInterval: 5 * time.Second, // long, so we observe the close before retry
		FrameTimeout:  500 * time.Millisecond,
	})
	defer s.Close()

	ctx := t.Context()
	ch := s.Subscribe(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel did not close after subprocess exit")
		}
	}
}

func TestStreamFrameTimeoutKillsHung(t *testing.T) {
	s := newTestStream(t, "hang-no-frames", Config{
		DevicePath:    "/dev/null",
		RetryInterval: 5 * time.Second,
		FrameTimeout:  150 * time.Millisecond,
	})
	defer s.Close()

	ctx := t.Context()
	ch := s.Subscribe(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel did not close after frame timeout")
		}
	}
}

func TestStreamFirstFrameTimeoutDistinct(t *testing.T) {
	logs := &captureHandler{}
	logger := slog.New(logs)
	cfg := Config{
		DevicePath:        "/dev/null",
		RetryInterval:     5 * time.Second,
		FirstFrameTimeout: 100 * time.Millisecond,
		FrameTimeout:      2 * time.Second,
	}
	s := New(cfg, logger)
	s.command = helperCommand(t, "hang-no-frames")
	defer s.Close()

	ctx := t.Context()
	ch := s.Subscribe(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				if !logs.findErr("no first frame within 100ms") {
					t.Fatal("expected first-frame timeout error in logs")
				}
				if logs.findErr("no frame within") {
					t.Fatal("steady-state timeout fired instead of first-frame timeout")
				}
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel did not close after first-frame timeout")
		}
	}
}

func TestStreamSteadyStateTimeoutAfterFirstFrame(t *testing.T) {
	logs := &captureHandler{}
	logger := slog.New(logs)
	cfg := Config{
		DevicePath:        "/dev/null",
		RetryInterval:     5 * time.Second,
		FirstFrameTimeout: 2 * time.Second,
		FrameTimeout:      150 * time.Millisecond,
	}
	s := New(cfg, logger)
	s.command = helperCommand(t, "one-frame-then-hang")
	defer s.Close()

	ctx := t.Context()
	ch := s.Subscribe(ctx)

	select {
	case vf, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before first frame")
		}
		if len(vf.Data) < 4 || vf.Data[0] != 0xFF || vf.Data[1] != 0xD8 {
			t.Fatalf("first frame not a JPEG: %x", vf.Data[:min(8, len(vf.Data))])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no first frame")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				if !logs.findErr("no frame within 150ms") {
					t.Fatal("expected steady-state timeout error in logs")
				}
				if logs.findErr("no first frame within") {
					t.Fatal("first-frame timeout fired instead of steady-state timeout")
				}
				return
			}
		case <-deadline:
			t.Fatal("subscriber channel did not close after steady-state timeout")
		}
	}
}

func TestStreamUnsubscribeOnContextCancel(t *testing.T) {
	s := newTestStream(t, "frames", Config{DevicePath: "/dev/null"})
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := s.Subscribe(ctx)
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("no first frame")
	}

	cancel()

	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("channel did not close after ctx cancel")
		}
	}
}

func TestStreamSatisfiesVideoSourceInterface(t *testing.T) {
	var _ kvm.VideoSource = (*MJPEGStream)(nil)
}
