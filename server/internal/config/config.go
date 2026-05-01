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
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object loaded from the on-disk YAML file.
type Config struct {
	HTTP     HTTPServerConfig `yaml:"http"`
	Auth     AuthConfig       `yaml:"auth"`
	Channels []ChannelConfig  `yaml:"channels"`
}

// HTTPServerConfig configures the HTTP listener.
type HTTPServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
}

// AuthConfig selects an auth provider by name; AuthProviderConfig is a raw
// JSON blob that the chosen provider unmarshals on its own.
type AuthConfig struct {
	AuthProvider       string          `yaml:"provider"`
	AuthProviderConfig json.RawMessage `yaml:"provider_config"`
}

// ChannelConfig binds a video source to a key event sink under a stable
// user-visible name. The Name is also used as the dispatch key when a Client
// switches channels.
type ChannelConfig struct {
	Name  string             `yaml:"name"`
	Video VideoSourceConfig  `yaml:"video"`
	Keys  KeyEventSinkConfig `yaml:"keys"`
}

// VideoSourceConfig selects a [kvm.VideoSource] driver by Type. DevicePath is
// driver-interpreted (e.g. /dev/video5 for v4l). Width/Height/Framerate are
// optional capture-side hints; zero means "let the driver pick the device
// default". Width and Height must be specified together.
type VideoSourceConfig struct {
	Type       string `yaml:"type"`
	DevicePath string `yaml:"device_path"`
	Width      int    `yaml:"width"`
	Height     int    `yaml:"height"`
	Framerate  int    `yaml:"framerate"`
}

// KeyEventSinkConfig selects a [kvm.KeyEventSink] driver by Type. DevicePath
// is driver-interpreted (e.g. /dev/ttyUSB0 for picolink).
type KeyEventSinkConfig struct {
	Type       string `yaml:"type"`
	DevicePath string `yaml:"device_path"`
}

// FromYAML parses the YAML file at path into c, replacing any prior contents.
func (c *Config) FromYAML(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from operator-supplied CLI flag
	if err != nil {
		return fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	return nil
}

// ToYAML serializes c to the file at path, overwriting it if present.
func (c *Config) ToYAML(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

// Validate returns the set of structural problems with c. An empty slice
// means c is acceptable. Validation is purely on the configuration shape; it
// does not probe devices or test reachability.
func (c *Config) Validate() []error {
	var errs []error
	if c.HTTP.ListenAddr == "" {
		errs = append(errs, errors.New("http.listen_addr must be set"))
	}
	if c.Auth.AuthProvider == "" {
		errs = append(errs, errors.New("auth.provider must be set"))
	}
	if len(c.Channels) == 0 {
		errs = append(errs, errors.New("at least one channel must be configured"))
	}
	seen := make(map[string]struct{}, len(c.Channels))
	for i, ch := range c.Channels {
		if ch.Name == "" {
			errs = append(errs, fmt.Errorf("channels[%d]: name must be set", i))
		} else if _, dup := seen[ch.Name]; dup {
			errs = append(errs, fmt.Errorf("channels[%d]: duplicate name %q", i, ch.Name))
		} else {
			seen[ch.Name] = struct{}{}
		}
		if ch.Video.Type == "" {
			errs = append(errs, fmt.Errorf("channels[%d].video.type must be set", i))
		}
		if ch.Video.Width < 0 {
			errs = append(errs, fmt.Errorf("channels[%d].video.width must be >= 0", i))
		}
		if ch.Video.Height < 0 {
			errs = append(errs, fmt.Errorf("channels[%d].video.height must be >= 0", i))
		}
		if ch.Video.Framerate < 0 {
			errs = append(errs, fmt.Errorf("channels[%d].video.framerate must be >= 0", i))
		}
		if (ch.Video.Width == 0) != (ch.Video.Height == 0) {
			errs = append(errs, fmt.Errorf("channels[%d].video: width and height must be specified together", i))
		}
		if ch.Keys.Type == "" {
			errs = append(errs, fmt.Errorf("channels[%d].keys.type must be set", i))
		}
	}
	return errs
}
