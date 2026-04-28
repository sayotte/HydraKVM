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

package websocket

import (
	"testing"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

func TestParseKeyCodeKnown(t *testing.T) {
	tr := NewW3CKeyEventTranslator()
	cases := []struct {
		s    string
		want kvm.KeyCode
	}{
		{"KeyA", kvm.KeyA},
		{"KeyZ", kvm.KeyZ},
		{"Digit0", kvm.Digit0},
		{"ShiftLeft", kvm.ShiftLeft},
		{"ShiftRight", kvm.ShiftRight},
		{"ControlLeft", kvm.ControlLeft},
		{"AltRight", kvm.AltRight},
		{"MetaLeft", kvm.MetaLeft},
		{"Enter", kvm.Enter},
		{"Escape", kvm.Escape},
		{"Tab", kvm.Tab},
		{"F1", kvm.F1},
		{"F12", kvm.F12},
		{"F24", kvm.F24},
		{"Numpad0", kvm.Numpad0},
		{"NumpadEnter", kvm.NumpadEnter},
		{"NumpadMemorySubtract", kvm.NumpadMemorySubtract},
		{"AudioVolumeUp", kvm.AudioVolumeUp},
		{"MediaPlayPause", kvm.MediaPlayPause},
		{"BrowserRefresh", kvm.BrowserRefresh},
		{"ArrowDown", kvm.ArrowDown},
		{"IntlYen", kvm.IntlYen},
	}
	for _, tc := range cases {
		t.Run(tc.s, func(t *testing.T) {
			got, ok := tr.ParseKeyCode(tc.s)
			if !ok {
				t.Fatalf("ParseKeyCode(%q): ok=false, want true", tc.s)
			}
			if got != tc.want {
				t.Fatalf("ParseKeyCode(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

func TestParseKeyCodeUnknown(t *testing.T) {
	tr := NewW3CKeyEventTranslator()
	for _, s := range []string{"", "junk", "keyA", "Key A", "KeyAA"} {
		got, ok := tr.ParseKeyCode(s)
		if ok {
			t.Errorf("ParseKeyCode(%q): ok=true, want false", s)
		}
		if got != kvm.KeyCodeReset {
			t.Errorf("ParseKeyCode(%q) = %v, want KeyCodeReset", s, got)
		}
	}
}

func TestParseKeyType(t *testing.T) {
	tr := NewW3CKeyEventTranslator()
	cases := []struct {
		s      string
		want   kvm.KeyType
		wantOK bool
	}{
		{"keydown", kvm.KeyTypeDown, true},
		{"keyup", kvm.KeyTypeUp, true},
		{"down", kvm.KeyTypeUp, false},
		{"up", kvm.KeyTypeUp, false},
		{"", kvm.KeyTypeUp, false},
		{"junk", kvm.KeyTypeUp, false},
	}
	for _, tc := range cases {
		t.Run(tc.s, func(t *testing.T) {
			got, ok := tr.ParseKeyType(tc.s)
			if ok != tc.wantOK {
				t.Fatalf("ParseKeyType(%q): ok=%v, want %v", tc.s, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("ParseKeyType(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}
