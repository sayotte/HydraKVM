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

import "testing"

func TestKeyCodeResetIsZeroValue(t *testing.T) {
	var zero KeyCode
	if zero != KeyCodeReset {
		t.Errorf("zero KeyCode = %d, want KeyCodeReset (%d)", zero, KeyCodeReset)
	}
}

func TestKeyTypeUpIsZeroValue(t *testing.T) {
	var zero KeyType
	if zero != KeyTypeUp {
		t.Errorf("zero KeyType = %d, want KeyTypeUp (%d)", zero, KeyTypeUp)
	}
}

type fakeTranslator struct{}

func (fakeTranslator) ParseKeyCode(s string) (KeyCode, bool) {
	if s == "KeyA" {
		return KeyA, true
	}
	return KeyCodeReset, false
}

func (fakeTranslator) ParseKeyType(s string) (KeyType, bool) {
	switch s {
	case "down":
		return KeyTypeDown, true
	case "up":
		return KeyTypeUp, true
	}
	return KeyTypeUp, false
}

func TestKeyEventTranslatorIsSatisfiable(t *testing.T) {
	var tr KeyEventTranslator = fakeTranslator{}
	code, ok := tr.ParseKeyCode("KeyA")
	if !ok || code != KeyA {
		t.Errorf("ParseKeyCode(KeyA) = (%d, %v); want (%d, true)", code, ok, KeyA)
	}
	kt, ok := tr.ParseKeyType("down")
	if !ok || kt != KeyTypeDown {
		t.Errorf("ParseKeyType(down) = (%d, %v); want (%d, true)", kt, ok, KeyTypeDown)
	}
}
