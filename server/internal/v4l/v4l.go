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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

// Config configures an [MJPEGStream]. DevicePath is the V4L2 device node
// (e.g. /dev/video0); Width/Height/Framerate are passed to ffmpeg as
// capture-side hints. Zero values for the hints mean "ask ffmpeg to use the
// device default".
type Config struct {
	DevicePath string
	Width      int
	Height     int
	Framerate  int

	// FFmpegPath overrides the resolved ffmpeg binary; empty uses the
	// PATH-resolved "ffmpeg".
	FFmpegPath string

	// RetryInterval is how long the driver waits between subprocess
	// relaunch attempts after a failure. Zero uses [defaultRetryInterval].
	RetryInterval time.Duration

	// FrameTimeout is the steady-state watchdog window: once the first
	// JPEG has arrived, if no further JPEG arrives within this duration the
	// driver considers the source dead, kills the process, and enters
	// retry. Zero falls back to 5 / Framerate seconds (or 5 /
	// [defaultFramerate] if Framerate is 0).
	FrameTimeout time.Duration

	// FirstFrameTimeout is the warmup window: how long the driver waits
	// between subprocess start and the first JPEG arriving before it gives
	// up. UVC devices often take 500ms-2s after STREAMON to deliver frame
	// 1, so this is intentionally looser than [Config.FrameTimeout]. Zero
	// uses [defaultFirstFrameTimeout].
	FirstFrameTimeout time.Duration

	// FFmpegLogLevel overrides the -loglevel passed to ffmpeg. Empty uses
	// "error" (quiet). Setting "info" or "verbose" surfaces device/format
	// negotiation chatter on ffmpeg's stderr, which the driver line-logs.
	FFmpegLogLevel string
}

const (
	defaultFFmpegBinary      = "ffmpeg"
	defaultRetryInterval     = 2 * time.Second
	defaultFramerate         = 30
	defaultFirstFrameTimeout = 3 * time.Second
	defaultFFmpegLogLevel    = "error"
	subprocessKillGrace      = 500 * time.Millisecond
)

// MJPEGStream is a [kvm.VideoSource] that wraps a long-running ffmpeg
// subprocess capturing MJPEG from a V4L2 device. Subscribers receive a
// per-call channel that is fed by the driver's fan-out goroutine; channels
// close (with ctx still live) when the subprocess fails, signalling source
// failure to upstream callers. Subsequent Subscribe calls produce fresh
// channels and trigger a relaunch attempt.
type MJPEGStream struct {
	cfg    Config
	logger *slog.Logger

	mu          sync.Mutex
	subscribers map[chan kvm.VideoFrame]struct{}
	supervised  bool
	stopping    bool
	stopFn      context.CancelFunc
	supDone     chan struct{}

	// hooks for tests; production runs always use realCommand.
	command commandFunc
}

// commandFunc is the seam tests use to inject a fake ffmpeg.
type commandFunc func(ctx context.Context, cfg Config) *exec.Cmd

func realCommand(ctx context.Context, cfg Config) *exec.Cmd {
	bin := cfg.FFmpegPath
	if bin == "" {
		bin = defaultFFmpegBinary
	}
	logLevel := cfg.FFmpegLogLevel
	if logLevel == "" {
		logLevel = defaultFFmpegLogLevel
	}
	args := []string{
		"-hide_banner",
		"-loglevel", logLevel,
		"-fflags", "nobuffer",
		"-thread_queue_size", "1",
		"-f", "v4l2",
		"-input_format", "mjpeg",
	}
	if cfg.Width > 0 && cfg.Height > 0 {
		args = append(args, "-video_size", strconv.Itoa(cfg.Width)+"x"+strconv.Itoa(cfg.Height))
	}
	if cfg.Framerate > 0 {
		args = append(args, "-framerate", strconv.Itoa(cfg.Framerate))
	}
	args = append(args,
		"-i", cfg.DevicePath,
		"-c", "copy",
		"-f", "mjpeg",
		"-",
	)
	return exec.CommandContext(ctx, bin, args...) //nolint:gosec // ffmpeg path and device are operator-supplied config, not user input
}

// New constructs an MJPEGStream from cfg. The subprocess is not started
// until the first call to [MJPEGStream.Subscribe].
func New(cfg Config, logger *slog.Logger) *MJPEGStream {
	if logger == nil {
		logger = slog.Default()
	}
	return &MJPEGStream{
		cfg:         cfg,
		logger:      logger,
		subscribers: make(map[chan kvm.VideoFrame]struct{}),
		command:     realCommand,
	}
}

// Shape reports the MJPEG stream parameters. Width/Height/Framerate echo
// the caller's hints — the driver does not parse JPEG headers to confirm
// negotiation, since [kvm.Application] is codec-agnostic and does not act
// on these fields.
func (s *MJPEGStream) Shape() kvm.StreamShape {
	return kvm.StreamShape{
		Codec:     "mjpeg",
		MIMEType:  "image/jpeg",
		Framing:   "multipart",
		Width:     s.cfg.Width,
		Height:    s.cfg.Height,
		Framerate: s.cfg.Framerate,
	}
}

// InitData returns nil; MJPEG has no codec init blob.
func (s *MJPEGStream) InitData() []byte { return nil }

// RequestKeyframe is a no-op: every MJPEG frame is a keyframe.
func (s *MJPEGStream) RequestKeyframe() error { return nil }

// Subscribe returns a fresh frame channel. The first call starts the
// supervisor goroutine; subsequent calls register additional fan-out
// recipients. When the last subscriber's ctx is cancelled the supervisor
// (and its ffmpeg subprocess) is torn down; the next Subscribe restarts
// it. The returned channel closes when ctx is cancelled, when the
// supervisor itself is stopped, or when a subprocess failure forces the
// driver to drop all subscribers.
func (s *MJPEGStream) Subscribe(ctx context.Context) <-chan kvm.VideoFrame {
	ch := make(chan kvm.VideoFrame, 1)
	s.mu.Lock()
	for s.stopping {
		done := s.supDone
		s.mu.Unlock()
		if done != nil {
			<-done
		}
		s.mu.Lock()
	}
	s.subscribers[ch] = struct{}{}
	if !s.supervised {
		s.supervised = true
		supCtx, cancel := context.WithCancel(context.Background())
		s.stopFn = cancel
		s.supDone = make(chan struct{})
		go s.supervise(supCtx)
	}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.unsubscribe(ch)
	}()

	return ch
}

// Close stops the supervisor and unblocks all subscribers. Idempotent.
func (s *MJPEGStream) Close() {
	s.mu.Lock()
	cancel := s.stopFn
	done := s.supDone
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *MJPEGStream) unsubscribe(ch chan kvm.VideoFrame) {
	s.mu.Lock()
	if _, ok := s.subscribers[ch]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.subscribers, ch)
	close(ch)
	var stop context.CancelFunc
	if len(s.subscribers) == 0 && s.supervised && !s.stopping {
		s.stopping = true
		stop = s.stopFn
	}
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (s *MJPEGStream) closeAllSubscribers() {
	s.mu.Lock()
	subs := s.subscribers
	s.subscribers = make(map[chan kvm.VideoFrame]struct{})
	s.mu.Unlock()
	for ch := range subs {
		close(ch)
	}
}

func (s *MJPEGStream) broadcast(vf kvm.VideoFrame) {
	s.mu.Lock()
	for ch := range s.subscribers {
		select {
		case ch <- vf:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *MJPEGStream) supervise(ctx context.Context) {
	defer func() {
		s.closeAllSubscribers()
		s.mu.Lock()
		s.supervised = false
		s.stopping = false
		s.stopFn = nil
		close(s.supDone)
		s.supDone = nil
		s.mu.Unlock()
	}()

	retry := s.cfg.RetryInterval
	if retry <= 0 {
		retry = defaultRetryInterval
	}

	for {
		if err := s.runOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			s.logger.Warn("v4l ffmpeg run ended",
				"device", s.cfg.DevicePath, "err", err)
			s.closeAllSubscribers()
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}

// runOnce launches one ffmpeg subprocess, pumps its stdout through the
// JPEG splitter, and broadcasts each frame. It returns when stdout EOFs,
// the watchdog fires, or ctx is cancelled. Stderr is line-logged on a
// helper goroutine.
func (s *MJPEGStream) runOnce(ctx context.Context) error {
	cmd := s.command(ctx, s.cfg)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}
	s.logger.Info("v4l ffmpeg started",
		"device", s.cfg.DevicePath, "pid", cmd.Process.Pid)

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.logger.Warn("v4l ffmpeg stderr",
				"device", s.cfg.DevicePath, "line", sc.Text())
		}
	}()

	frameTimeout := s.cfg.FrameTimeout
	if frameTimeout <= 0 {
		fr := s.cfg.Framerate
		if fr <= 0 {
			fr = defaultFramerate
		}
		frameTimeout = max(5*time.Second/time.Duration(fr), 200*time.Millisecond)
	}
	firstFrameTimeout := s.cfg.FirstFrameTimeout
	if firstFrameTimeout <= 0 {
		firstFrameTimeout = defaultFirstFrameTimeout
	}

	pumpErr := s.pump(ctx, stdout, firstFrameTimeout, frameTimeout)

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	if cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	select {
	case <-waitDone:
	case <-time.After(subprocessKillGrace):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitDone
	}
	<-stderrDone
	s.logger.Info("v4l ffmpeg exited",
		"device", s.cfg.DevicePath, "pid", cmd.Process.Pid)

	if ctx.Err() != nil {
		return context.Canceled
	}
	return pumpErr
}

// pump reads JPEGs from r, broadcasts them, and arms a watchdog timer
// against frame arrivals. The watchdog uses firstFrameTimeout until the
// first JPEG lands and frameTimeout thereafter; UVC devices commonly take
// hundreds of ms to a few seconds after STREAMON before the first frame,
// so the warmup window is intentionally looser than the steady-state
// inter-frame budget. Returns nil-or-error explaining why the loop
// stopped: ctx cancellation, watchdog timeout, or a reader error.
func (s *MJPEGStream) pump(ctx context.Context, r io.Reader, firstFrameTimeout, frameTimeout time.Duration) error {
	type frameOrErr struct {
		data []byte
		err  error
	}
	splitter := newJPEGSplitter(r)
	frames := make(chan frameOrErr, 1)
	pumpCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			data, err := splitter.next()
			select {
			case <-pumpCtx.Done():
				return
			case frames <- frameOrErr{data: data, err: err}:
			}
			if err != nil {
				return
			}
		}
	}()

	startedAt := time.Now()
	gotFirstFrame := false
	timer := time.NewTimer(firstFrameTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-timer.C:
			if !gotFirstFrame {
				return fmt.Errorf("v4l: no first frame within %s", firstFrameTimeout)
			}
			return fmt.Errorf("v4l: no frame within %s", frameTimeout)
		case f := <-frames:
			if f.err != nil {
				if errors.Is(f.err, io.EOF) {
					return io.EOF
				}
				return f.err
			}
			s.broadcast(kvm.VideoFrame{
				Data:  f.data,
				IsKey: true,
				PTS:   time.Since(startedAt),
			})
			gotFirstFrame = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(frameTimeout)
		}
	}
}
