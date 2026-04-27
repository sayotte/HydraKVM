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

func TestKeyboardSatisfiesInterface(t *testing.T) {
	var _ kvm.KeyEventSink = NewKeyboard("/dev/null")
}

func TestReportKeyEventDoesNotPanic(t *testing.T) {
	k := NewKeyboard("/dev/null")
	k.ReportKeyEvent(kvm.KeyEvent{Code: "KeyA", Kind: "down"})
	k.ReportKeyEvent(kvm.KeyEvent{Code: "KeyA", Kind: "up"})
}
