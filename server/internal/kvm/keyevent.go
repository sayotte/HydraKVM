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

package kvm

// ModifierMask is the USB HID boot-keyboard modifier byte: a bitmap of the
// eight modifier keys held at a given instant.
type ModifierMask uint8

const (
	ModLeftCtrl   ModifierMask = 1 << 0
	ModLeftShift  ModifierMask = 1 << 1
	ModLeftAlt    ModifierMask = 1 << 2
	ModLeftMeta   ModifierMask = 1 << 3
	ModRightCtrl  ModifierMask = 1 << 4
	ModRightShift ModifierMask = 1 << 5
	ModRightAlt   ModifierMask = 1 << 6
	ModRightMeta  ModifierMask = 1 << 7
)

// KeyEvent is a single edge transition decided by the Application and handed
// to a KeyEventSink. It carries the canonical kvm-side enums; wire-side
// string parsing happens upstream via a [KeyEventTranslator].
type KeyEvent struct {
	Code KeyCode
	Type KeyType
	// Modifiers is the post-edge snapshot of held modifiers, including this
	// edge if Code is itself a modifier key.
	Modifiers ModifierMask
}

// KeyboardState tracks the per-Channel keyboard view. Mutated by Application;
// the sink is stateless with respect to this data.
type KeyboardState struct {
	Modifiers ModifierMask
}
