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

package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
)

//go:embed index.html.tmpl
var indexTemplateFS embed.FS

// all: prefix is required so that the placeholder .gitkeep (and any other
// dotfiles emitted by the build) are included; without it embed silently
// skips files whose names start with '.' or '_', and an empty dist directory
// fails to embed at all.
//
//go:embed all:dist
var distFS embed.FS

// IndexData is the template context consumed by the embedded index template.
// Sub-step 5 will extend the shape; sub-step 3 ships the minimum needed to
// render a channel-selector skeleton.
type IndexData struct {
	Title    string
	Channels []ChannelInfo
}

// ChannelInfo is one entry in the rendered channel selector.
type ChannelInfo struct {
	ID   string
	Name string
}

// IndexTemplate parses the embedded index.html.tmpl. Returns a fresh
// [*template.Template] each call; callers are expected to parse once at
// server construction.
func IndexTemplate() (*template.Template, error) {
	t, err := template.ParseFS(indexTemplateFS, "index.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("web: parse index template: %w", err)
	}
	return t, nil
}

// StaticFS returns the bundled `dist/` subtree as an [fs.FS]. Callers mount
// it under `/static/`.
func StaticFS() (fs.FS, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("web: open dist subtree: %w", err)
	}
	return sub, nil
}
