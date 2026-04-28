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
	"bytes"
	"strings"
	"testing"
)

func TestIndexTemplateRenders(t *testing.T) {
	tmpl, err := IndexTemplate()
	if err != nil {
		t.Fatalf("IndexTemplate: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, IndexData{
		Title:    "HydraKVM",
		Channels: []ChannelInfo{{ID: "ch1", Name: "Channel One"}},
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `<option value="ch1">`) {
		t.Errorf("rendered output missing channel option; got:\n%s", out)
	}
	if !strings.Contains(out, "<title>HydraKVM</title>") {
		t.Errorf("rendered output missing title; got:\n%s", out)
	}
}

func TestStaticFSOpens(t *testing.T) {
	sfs, err := StaticFS()
	if err != nil {
		t.Fatalf("StaticFS: %v", err)
	}
	if sfs == nil {
		t.Fatal("StaticFS returned nil fs.FS")
	}
	f, err := sfs.Open(".gitkeep")
	if err != nil {
		t.Fatalf("open .gitkeep: %v", err)
	}
	_ = f.Close()
}
