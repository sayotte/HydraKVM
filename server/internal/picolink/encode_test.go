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

package picolink

import (
	"testing"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

func TestEncodeKeyEventRegularKeyDown(t *testing.T) {
	got, ok := EncodeKeyEvent(kvm.KeyEvent{Code: kvm.KeyA, Type: kvm.KeyTypeDown})
	if !ok {
		t.Fatalf("EncodeKeyEvent(KeyA down) ok=false, want true")
	}
	want := [3]byte{0xFF, 0x00, 0x04}
	if got != want {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventRegularKeyUp(t *testing.T) {
	got, ok := EncodeKeyEvent(kvm.KeyEvent{Code: kvm.KeyA, Type: kvm.KeyTypeUp})
	if !ok {
		t.Fatalf("ok=false")
	}
	want := [3]byte{0xFF, 0x00, 0x00}
	if got != want {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventModifierOnlyEdge(t *testing.T) {
	// ShiftLeft down: kc must be 0; mod byte carries the shift bit.
	got, ok := EncodeKeyEvent(kvm.KeyEvent{
		Code:      kvm.ShiftLeft,
		Type:      kvm.KeyTypeDown,
		Modifiers: kvm.ModLeftShift,
	})
	if !ok {
		t.Fatalf("ok=false")
	}
	want := [3]byte{0xFF, byte(kvm.ModLeftShift), 0x00}
	if got != want {
		t.Fatalf("ShiftLeft down frame = %#v, want %#v", got, want)
	}

	// ShiftLeft up: kc still 0; mod byte cleared.
	got, ok = EncodeKeyEvent(kvm.KeyEvent{
		Code:      kvm.ShiftLeft,
		Type:      kvm.KeyTypeUp,
		Modifiers: 0,
	})
	if !ok {
		t.Fatalf("ok=false")
	}
	want = [3]byte{0xFF, 0x00, 0x00}
	if got != want {
		t.Fatalf("ShiftLeft up frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventKeyWithModifier(t *testing.T) {
	// Alt+A down — mod=Alt, kc=HID_A.
	got, ok := EncodeKeyEvent(kvm.KeyEvent{
		Code:      kvm.KeyA,
		Type:      kvm.KeyTypeDown,
		Modifiers: kvm.ModLeftAlt,
	})
	if !ok {
		t.Fatalf("ok=false")
	}
	want := [3]byte{0xFF, byte(kvm.ModLeftAlt), 0x04}
	if got != want {
		t.Fatalf("Alt+A down frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventKeyReleaseWithModifierHeld(t *testing.T) {
	// A up while Alt still held — mod=Alt, kc=0.
	got, ok := EncodeKeyEvent(kvm.KeyEvent{
		Code:      kvm.KeyA,
		Type:      kvm.KeyTypeUp,
		Modifiers: kvm.ModLeftAlt,
	})
	if !ok {
		t.Fatalf("ok=false")
	}
	want := [3]byte{0xFF, byte(kvm.ModLeftAlt), 0x00}
	if got != want {
		t.Fatalf("A up (Alt held) frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventUnknownKeyCode(t *testing.T) {
	// KeyCodeReset has no HID mapping.
	if _, ok := EncodeKeyEvent(kvm.KeyEvent{Code: kvm.KeyCodeReset, Type: kvm.KeyTypeDown}); ok {
		t.Fatalf("expected ok=false for KeyCodeReset")
	}
	// A high media key without an entry in the table.
	if _, ok := EncodeKeyEvent(kvm.KeyEvent{Code: kvm.BrowserBack}); ok {
		t.Fatalf("expected ok=false for BrowserBack")
	}
}

func TestEncodeKeyEventAllModifierBits(t *testing.T) {
	// All eight modifiers held simultaneously, regular key down.
	all := kvm.ModLeftCtrl | kvm.ModLeftShift | kvm.ModLeftAlt | kvm.ModLeftMeta |
		kvm.ModRightCtrl | kvm.ModRightShift | kvm.ModRightAlt | kvm.ModRightMeta
	got, ok := EncodeKeyEvent(kvm.KeyEvent{
		Code:      kvm.KeyZ,
		Type:      kvm.KeyTypeDown,
		Modifiers: all,
	})
	if !ok {
		t.Fatalf("ok=false")
	}
	want := [3]byte{0xFF, 0xFF, 0x1D}
	if got != want {
		t.Fatalf("all-mods+Z frame = %#v, want %#v", got, want)
	}
}

func TestEncodeKeyEventLetterCoverage(t *testing.T) {
	cases := []struct {
		code kvm.KeyCode
		hid  byte
	}{
		{kvm.KeyA, 0x04}, {kvm.KeyZ, 0x1D},
		{kvm.Digit1, 0x1E}, {kvm.Digit0, 0x27},
		{kvm.Enter, 0x28}, {kvm.Escape, 0x29},
		{kvm.Space, 0x2C}, {kvm.Tab, 0x2B},
		{kvm.F1, 0x3A}, {kvm.F12, 0x45},
		{kvm.ArrowUp, 0x52}, {kvm.ArrowDown, 0x51},
		{kvm.ArrowLeft, 0x50}, {kvm.ArrowRight, 0x4F},
		{kvm.Numpad0, 0x62}, {kvm.NumpadDecimal, 0x63},
	}
	for _, c := range cases {
		got, ok := EncodeKeyEvent(kvm.KeyEvent{Code: c.code, Type: kvm.KeyTypeDown})
		if !ok {
			t.Errorf("code=%v ok=false", c.code)
			continue
		}
		if got[2] != c.hid {
			t.Errorf("code=%v kc=0x%02X, want 0x%02X", c.code, got[2], c.hid)
		}
	}
}
