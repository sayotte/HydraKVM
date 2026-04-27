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
	"errors"
	"flag"
	"fmt"
	"io"
	nethttp "net/http"
	"os"
	"path/filepath"

	"github.com/sayotte/hydrakvm/internal/auth"
	"github.com/sayotte/hydrakvm/internal/config"
	"github.com/sayotte/hydrakvm/internal/dispatch"
	hkhttp "github.com/sayotte/hydrakvm/internal/http"
	"github.com/sayotte/hydrakvm/internal/kvm"
	"github.com/sayotte/hydrakvm/internal/picolink"
	"github.com/sayotte/hydrakvm/internal/synthetic"
)

const defaultConfigPath = "/etc/hydrakvm/config.yaml"

func main() {
	cfgPath := flag.String("config", defaultConfigPath, "path to the HydraKVM YAML configuration file")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = io.WriteString(out, "hydrakvm — HydraKVM server\n\nUsage: ")
		_, _ = io.WriteString(out, filepath.Base(os.Args[0])) //nolint:gosec // CLI usage banner; not a web sink
		_, _ = io.WriteString(out, " [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		fmt.Fprintln(os.Stderr, "hydrakvm:", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	var cfg config.Config
	if err := cfg.FromYAML(cfgPath); err != nil {
		return err
	}
	if errs := cfg.Validate(); len(errs) > 0 {
		return fmt.Errorf("config invalid: %w", errors.Join(errs...))
	}

	authProvider, err := newAuthProvider(cfg.Auth)
	if err != nil {
		return err
	}

	app := kvm.NewApplication(context.Background())
	for _, chCfg := range cfg.Channels {
		ch, err := newChannel(chCfg)
		if err != nil {
			return fmt.Errorf("channel %q: %w", chCfg.Name, err)
		}
		app.AddChannel(chCfg.Name, ch)
	}

	router := dispatch.NewRouter()
	dispatch.Register(router, kvm.MsgSwitchChannel, app.SwitchChannel)
	dispatch.RegisterNotification(router, kvm.MsgKeyEvent, app.RecordKeyEvent)

	server := hkhttp.NewServer(cfg.HTTP, authProvider, router)
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

func newChannel(cfg config.ChannelConfig) (*kvm.Channel, error) {
	var vs kvm.VideoSource
	switch cfg.Video.Type {
	case "synthetic":
		vs = synthetic.NewVideoSource()
	default:
		return nil, fmt.Errorf("unknown video source type %q", cfg.Video.Type)
	}

	var ks kvm.KeyEventSink
	switch cfg.Keys.Type {
	case "picolink":
		ks = picolink.NewKeyboard(cfg.Keys.DevicePath)
	default:
		return nil, fmt.Errorf("unknown key sink type %q", cfg.Keys.Type)
	}

	return kvm.NewChannel(vs, ks), nil
}
