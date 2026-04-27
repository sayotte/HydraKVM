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

import "github.com/sayotte/hydrakvm/internal/kvm"

// Keyboard is the stub [kvm.KeyEventSink] for the Pico serial link. It
// silently discards every event; serial framing and HID translation arrive in
// Step 4.
type Keyboard struct {
	devicePath string
}

// NewKeyboard constructs a stub Keyboard. The devicePath is recorded but not
// opened.
func NewKeyboard(devicePath string) *Keyboard {
	return &Keyboard{devicePath: devicePath}
}

// ReportKeyEvent discards ke.
func (k *Keyboard) ReportKeyEvent(_ kvm.KeyEvent) {}
