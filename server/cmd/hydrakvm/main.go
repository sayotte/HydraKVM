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

// Command hydrakvm is the HydraKVM server entrypoint: it loads configuration,
// constructs the dispatch graph (Application + Channels + drivers), and runs
// the HTTP front end. Wiring lives here so that domain packages
// ([github.com/sayotte/hydrakvm/internal/kvm] and friends) stay free of
// dependency-injection boilerplate.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	nethttp "net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	hkhttp "github.com/sayotte/hydrakvm/internal/http"
	wsockt "github.com/sayotte/hydrakvm/internal/http/websocket"
	"github.com/sayotte/hydrakvm/internal/kvm"
	"github.com/sayotte/hydrakvm/internal/picolink"
	"github.com/sayotte/hydrakvm/internal/synthetic"
	"github.com/sayotte/hydrakvm/internal/v4l"
)

const (
	defaultConfigPath = "/etc/hydrakvm/config.yaml"
	shutdownTimeout   = 10 * time.Second
)

func main() {
	cfgPath := flag.String("config", defaultConfigPath, "path to the HydraKVM YAML configuration file")
	logLevel := flag.String("log-level", "info", "log verbosity: debug | info | warn | error")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = io.WriteString(out, "hydrakvm — HydraKVM server\n\nUsage: ")
		_, _ = io.WriteString(out, filepath.Base(os.Args[0])) //nolint:gosec // CLI usage banner; not a web sink
		_, _ = io.WriteString(out, " [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(*cfgPath, *logLevel); err != nil {
		fmt.Fprintln(os.Stderr, "hydrakvm:", err)
		os.Exit(1)
	}
}

func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (want debug|info|warn|error)", s)
	}
}

func run(cfgPath, logLevel string) error {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		return err
	}

	var cfg config.Config
	if err := cfg.FromYAML(cfgPath); err != nil {
		return err
	}
	if errs := cfg.Validate(); len(errs) > 0 {
		return fmt.Errorf("config invalid: %w", errors.Join(errs...))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	authProvider, err := newAuthProvider(cfg.Auth)
	if err != nil {
		return err
	}

	app := kvm.NewApplication(context.Background(), logger)
	for _, chCfg := range cfg.Channels {
		ch, err := newChannel(chCfg, logger)
		if err != nil {
			return fmt.Errorf("channel %q: %w", chCfg.Name, err)
		}
		app.AddChannel(chCfg.Name, ch)
	}

	// NoChannelVideo feeds an unattached Client's video pipe before any
	// SwitchChannel selects a real Channel and after a switch back to the
	// "(none)" sentinel; supplied as an interface value so internal/kvm
	// need not import internal/synthetic.
	app.NoChannelVideo = synthetic.NewVideoSource("No Channel Selected")

	// FallbackVideo covers any Channel whose primary VideoIn fails.
	app.FallbackVideo = synthetic.NewVideoSource("Channel Down")

	tr := wsockt.NewW3CKeyEventTranslator()
	router := dispatch.NewRouter()
	dispatch.Register(router, kvm.MsgSwitchChannel, app.SwitchChannel)
	router.Handle(kvm.MsgKeyEvent, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var p wsockt.KeyEventParams
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("decode key event: %w", err)
		}
		code, ok := tr.ParseKeyCode(p.Code)
		if !ok {
			return nil, fmt.Errorf("unknown key code %q", p.Code)
		}
		typ, ok := tr.ParseKeyType(p.Type)
		if !ok {
			return nil, fmt.Errorf("unknown key type %q", p.Type)
		}
		return nil, app.RecordKeyEvent(ctx, kvm.KeyEventParams{Code: code, Type: typ})
	})

	server, err := hkhttp.NewServer(cfg.HTTP, authProvider, router, app, logger)
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
		return err
	}
	return nil
}

func newAuthProvider(cfg config.AuthConfig) (auth.Provider, error) {
	switch cfg.AuthProvider {
	case "null":
		return auth.NewNullProvider(cfg.AuthProviderConfig)
	default:
		return nil, fmt.Errorf("unknown auth provider %q", cfg.AuthProvider)
	}
}

func newChannel(cfg config.ChannelConfig, logger *slog.Logger) (*kvm.Channel, error) {
	var vs kvm.VideoSource
	switch cfg.Video.Type {
	case "synthetic":
		vs = synthetic.NewVideoSource(cfg.Name)
	case "v4l":
		vs = v4l.New(v4l.Config{
			DevicePath: cfg.Video.DevicePath,
			Width:      cfg.Video.Width,
			Height:     cfg.Video.Height,
			Framerate:  cfg.Video.Framerate,
		}, logger)
	default:
		return nil, fmt.Errorf("unknown video source type %q", cfg.Video.Type)
	}

	var ks kvm.KeyEventSink
	switch cfg.Keys.Type {
	case "picolink":
		kb, err := picolink.NewKeyboard(cfg.Keys.DevicePath, logger)
		if err != nil {
			return nil, err
		}
		ks = kb
	case "null":
		ks = loggingDiscardKeyboard{name: cfg.Name, logger: logger}
	default:
		return nil, fmt.Errorf("unknown key sink type %q", cfg.Keys.Type)
	}

	return kvm.NewChannel(vs, ks), nil
}

// loggingDiscardKeyboard is the "null" key-sink driver: it discards events
// like discardKeyboard but logs each one at debug, useful for end-to-end
// dispatch troubleshooting without real serial hardware.
type loggingDiscardKeyboard struct {
	name   string
	logger *slog.Logger
}

func (k loggingDiscardKeyboard) ReportKeyEvent(ke kvm.KeyEvent) {
	k.logger.Debug("null key sink",
		"channel", k.name,
		"code", ke.Code,
		"type", ke.Type,
		"modifiers", fmt.Sprintf("%#x", ke.Modifiers))
}
