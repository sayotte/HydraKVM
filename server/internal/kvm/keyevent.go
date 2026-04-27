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

// KeyEvent is a single edge transition decided by the Application and handed
// to a KeyEventSink. Code uses KeyboardEvent.code identifiers ("KeyA",
// "ShiftLeft", "F11"); translation to USB HID usages is the sink's job.
type KeyEvent struct {
	Code string
	Kind string // "down" | "up"
}

// KeyboardState tracks the per-Channel keyboard view: held modifiers, the set
// of currently-pressed usages, and any policy flags that influence how the
// Application drives the Channel's KeyEventSink. Mutated by Application; the
// sink is stateless with respect to this data.
type KeyboardState struct {
	// Concrete fields are added in Step 4, when keyevent decoding lands.
}
