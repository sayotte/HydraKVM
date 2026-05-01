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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goldenYAML = `http:
  listen_addr: ":8080"
auth:
  provider: "null"
channels:
  - name: "synth-1"
    video:
      type: "synthetic"
    keys:
      type: "picolink"
      device_path: "/dev/ttyUSB0"
`

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestFromYAMLParsesGoldenConfig(t *testing.T) {
	path := writeTemp(t, "config.yaml", goldenYAML)
	var c Config
	if err := c.FromYAML(path); err != nil {
		t.Fatalf("FromYAML: %v", err)
	}
	if c.HTTP.ListenAddr != ":8080" {
		t.Errorf("listen_addr = %q, want :8080", c.HTTP.ListenAddr)
	}
	if c.Auth.AuthProvider != "null" {
		t.Errorf("auth.provider = %q, want null", c.Auth.AuthProvider)
	}
	if got := len(c.Channels); got != 1 {
		t.Fatalf("len(channels) = %d, want 1", got)
	}
	ch := c.Channels[0]
	if ch.Name != "synth-1" || ch.Video.Type != "synthetic" || ch.Keys.Type != "picolink" {
		t.Errorf("channel mismatch: %+v", ch)
	}
	if ch.Keys.DevicePath != "/dev/ttyUSB0" {
		t.Errorf("device_path = %q", ch.Keys.DevicePath)
	}
}

func TestFromYAMLReportsMissingFile(t *testing.T) {
	var c Config
	if err := c.FromYAML(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFromYAMLReportsParseError(t *testing.T) {
	path := writeTemp(t, "bad.yaml", "this: is: not: valid")
	var c Config
	if err := c.FromYAML(path); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestRoundTripYAML(t *testing.T) {
	c := Config{
		HTTP: HTTPServerConfig{ListenAddr: ":1234"},
		Auth: AuthConfig{AuthProvider: "null"},
		Channels: []ChannelConfig{{
			Name:  "a",
			Video: VideoSourceConfig{Type: "synthetic"},
			Keys:  KeyEventSinkConfig{Type: "picolink", DevicePath: "/dev/x"},
		}},
	}
	path := filepath.Join(t.TempDir(), "rt.yaml")
	if err := c.ToYAML(path); err != nil {
		t.Fatalf("ToYAML: %v", err)
	}
	var got Config
	if err := got.FromYAML(path); err != nil {
		t.Fatalf("FromYAML: %v", err)
	}
	if got.HTTP != c.HTTP || got.Auth.AuthProvider != c.Auth.AuthProvider || len(got.Channels) != 1 || got.Channels[0] != c.Channels[0] {
		t.Errorf("round-trip mismatch:\n  in:  %+v\n  out: %+v", c, got)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	var c Config
	errs := c.Validate()
	if len(errs) == 0 {
		t.Fatal("expected validation errors for zero Config")
	}
}

func TestValidateRejectsDuplicateChannelNames(t *testing.T) {
	c := Config{
		HTTP: HTTPServerConfig{ListenAddr: ":1"},
		Auth: AuthConfig{AuthProvider: "null"},
		Channels: []ChannelConfig{
			{Name: "dup", Video: VideoSourceConfig{Type: "synthetic"}, Keys: KeyEventSinkConfig{Type: "picolink"}},
			{Name: "dup", Video: VideoSourceConfig{Type: "synthetic"}, Keys: KeyEventSinkConfig{Type: "picolink"}},
		},
	}
	errs := c.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-name error, got: %v", errs)
	}
}

func TestValidateVideoSizeAndFramerate(t *testing.T) {
	base := func() Config {
		return Config{
			HTTP: HTTPServerConfig{ListenAddr: ":1"},
			Auth: AuthConfig{AuthProvider: "null"},
			Channels: []ChannelConfig{
				{Name: "ok", Video: VideoSourceConfig{Type: "v4l"}, Keys: KeyEventSinkConfig{Type: "picolink"}},
			},
		}
	}

	t.Run("zeros accepted", func(t *testing.T) {
		c := base()
		if errs := c.Validate(); len(errs) != 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
	})

	t.Run("positives accepted", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Width = 1920
		c.Channels[0].Video.Height = 1080
		c.Channels[0].Video.Framerate = 30
		if errs := c.Validate(); len(errs) != 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
	})

	t.Run("framerate alone accepted", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Framerate = 60
		if errs := c.Validate(); len(errs) != 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
	})

	t.Run("negative width rejected", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Width = -1
		c.Channels[0].Video.Height = 1080
		errs := c.Validate()
		if !anyContains(errs, "width must be >= 0") {
			t.Errorf("expected width >=0 error, got: %v", errs)
		}
	})

	t.Run("negative height rejected", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Width = 1920
		c.Channels[0].Video.Height = -1
		errs := c.Validate()
		if !anyContains(errs, "height must be >= 0") {
			t.Errorf("expected height >=0 error, got: %v", errs)
		}
	})

	t.Run("negative framerate rejected", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Framerate = -1
		errs := c.Validate()
		if !anyContains(errs, "framerate must be >= 0") {
			t.Errorf("expected framerate >=0 error, got: %v", errs)
		}
	})

	t.Run("width without height rejected", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Width = 1920
		errs := c.Validate()
		if !anyContains(errs, "width and height must be specified together") {
			t.Errorf("expected paired error, got: %v", errs)
		}
	})

	t.Run("height without width rejected", func(t *testing.T) {
		c := base()
		c.Channels[0].Video.Height = 1080
		errs := c.Validate()
		if !anyContains(errs, "width and height must be specified together") {
			t.Errorf("expected paired error, got: %v", errs)
		}
	})
}

func anyContains(errs []error, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), sub) {
			return true
		}
	}
	return false
}

func TestValidateAcceptsMinimalConfig(t *testing.T) {
	c := Config{
		HTTP: HTTPServerConfig{ListenAddr: ":1"},
		Auth: AuthConfig{AuthProvider: "null"},
		Channels: []ChannelConfig{
			{Name: "ok", Video: VideoSourceConfig{Type: "synthetic"}, Keys: KeyEventSinkConfig{Type: "picolink"}},
		},
	}
	if errs := c.Validate(); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
