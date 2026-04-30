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

//go:build hardware

package picolink

import (
	"os"
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

// TestHardwareTypeHelloWorld opens a real Pico serial port and emits a short
// canned sequence: "Hello, World!" followed by Ctrl-Alt-Delete. Run with:
//
//	HYDRAKVM_PICO_TTY=/dev/ttyACM0 go test -tags=hardware ./internal/picolink
func TestHardwareTypeHelloWorld(t *testing.T) {
	dev := os.Getenv("HYDRAKVM_PICO_TTY")
	if dev == "" {
		t.Skipf("HYDRAKVM_PICO_TTY not set; skipping hardware test")
	}

	kb, err := NewKeyboard(dev, nil)
	if err != nil {
		t.Fatalf("NewKeyboard(%q): %v", dev, err)
	}
	defer kb.Close()

	// Allow the firmware a moment to settle if the device just enumerated.
	time.Sleep(250 * time.Millisecond)

	type step struct {
		code kvm.KeyCode
		typ  kvm.KeyType
		mods kvm.ModifierMask
	}

	tap := func(out *[]step, c kvm.KeyCode, mods kvm.ModifierMask) {
		*out = append(*out,
			step{c, kvm.KeyTypeDown, mods},
			step{c, kvm.KeyTypeUp, mods},
		)
	}
	shifted := func(out *[]step, c kvm.KeyCode) {
		*out = append(*out,
			step{kvm.ShiftLeft, kvm.KeyTypeDown, kvm.ModLeftShift},
			step{c, kvm.KeyTypeDown, kvm.ModLeftShift},
			step{c, kvm.KeyTypeUp, kvm.ModLeftShift},
			step{kvm.ShiftLeft, kvm.KeyTypeUp, 0},
		)
	}

	var steps []step
	// "Hello, World!"
	shifted(&steps, kvm.KeyH)
	tap(&steps, kvm.KeyE, 0)
	tap(&steps, kvm.KeyL, 0)
	tap(&steps, kvm.KeyL, 0)
	tap(&steps, kvm.KeyO, 0)
	tap(&steps, kvm.Comma, 0)
	tap(&steps, kvm.Space, 0)
	shifted(&steps, kvm.KeyW)
	tap(&steps, kvm.KeyO, 0)
	tap(&steps, kvm.KeyR, 0)
	tap(&steps, kvm.KeyL, 0)
	tap(&steps, kvm.KeyD, 0)
	shifted(&steps, kvm.Digit1)

	// Ctrl-Alt-Delete.
	steps = append(steps,
		step{kvm.ControlLeft, kvm.KeyTypeDown, kvm.ModLeftCtrl},
		step{kvm.AltLeft, kvm.KeyTypeDown, kvm.ModLeftCtrl | kvm.ModLeftAlt},
		step{kvm.Delete, kvm.KeyTypeDown, kvm.ModLeftCtrl | kvm.ModLeftAlt},
		step{kvm.Delete, kvm.KeyTypeUp, kvm.ModLeftCtrl | kvm.ModLeftAlt},
		step{kvm.AltLeft, kvm.KeyTypeUp, kvm.ModLeftCtrl},
		step{kvm.ControlLeft, kvm.KeyTypeUp, 0},
	)

	for _, s := range steps {
		kb.ReportKeyEvent(kvm.KeyEvent{Code: s.code, Type: s.typ, Modifiers: s.mods})
		time.Sleep(15 * time.Millisecond)
	}
}
