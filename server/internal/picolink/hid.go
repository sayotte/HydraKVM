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

// hidUsage maps a [kvm.KeyCode] to its USB HID Usage Page 0x07 (Keyboard /
// Keypad) usage ID. Modifier keys map to their 0xE0..0xE7 usage but the
// encoder never emits those in the keycode byte; the modifier byte is taken
// from [kvm.KeyEvent.Modifiers] instead. Unmapped keys return 0.
var hidUsage = map[kvm.KeyCode]uint8{
	kvm.KeyA: 0x04, kvm.KeyB: 0x05, kvm.KeyC: 0x06, kvm.KeyD: 0x07,
	kvm.KeyE: 0x08, kvm.KeyF: 0x09, kvm.KeyG: 0x0A, kvm.KeyH: 0x0B,
	kvm.KeyI: 0x0C, kvm.KeyJ: 0x0D, kvm.KeyK: 0x0E, kvm.KeyL: 0x0F,
	kvm.KeyM: 0x10, kvm.KeyN: 0x11, kvm.KeyO: 0x12, kvm.KeyP: 0x13,
	kvm.KeyQ: 0x14, kvm.KeyR: 0x15, kvm.KeyS: 0x16, kvm.KeyT: 0x17,
	kvm.KeyU: 0x18, kvm.KeyV: 0x19, kvm.KeyW: 0x1A, kvm.KeyX: 0x1B,
	kvm.KeyY: 0x1C, kvm.KeyZ: 0x1D,

	kvm.Digit1: 0x1E, kvm.Digit2: 0x1F, kvm.Digit3: 0x20, kvm.Digit4: 0x21,
	kvm.Digit5: 0x22, kvm.Digit6: 0x23, kvm.Digit7: 0x24, kvm.Digit8: 0x25,
	kvm.Digit9: 0x26, kvm.Digit0: 0x27,

	kvm.Enter:        0x28,
	kvm.Escape:       0x29,
	kvm.Backspace:    0x2A,
	kvm.Tab:          0x2B,
	kvm.Space:        0x2C,
	kvm.Minus:        0x2D,
	kvm.Equal:        0x2E,
	kvm.BracketLeft:  0x2F,
	kvm.BracketRight: 0x30,
	kvm.Backslash:    0x31,
	kvm.Semicolon:    0x33,
	kvm.Quote:        0x34,
	kvm.Backquote:    0x35,
	kvm.Comma:        0x36,
	kvm.Period:       0x37,
	kvm.Slash:        0x38,
	kvm.CapsLock:     0x39,

	kvm.F1: 0x3A, kvm.F2: 0x3B, kvm.F3: 0x3C, kvm.F4: 0x3D,
	kvm.F5: 0x3E, kvm.F6: 0x3F, kvm.F7: 0x40, kvm.F8: 0x41,
	kvm.F9: 0x42, kvm.F10: 0x43, kvm.F11: 0x44, kvm.F12: 0x45,

	kvm.PrintScreen: 0x46,
	kvm.ScrollLock:  0x47,
	kvm.Pause:       0x48,
	kvm.Insert:      0x49,
	kvm.Home:        0x4A,
	kvm.PageUp:      0x4B,
	kvm.Delete:      0x4C,
	kvm.End:         0x4D,
	kvm.PageDown:    0x4E,
	kvm.ArrowRight:  0x4F,
	kvm.ArrowLeft:   0x50,
	kvm.ArrowDown:   0x51,
	kvm.ArrowUp:     0x52,

	kvm.NumLock:          0x53,
	kvm.NumpadDivide:     0x54,
	kvm.NumpadMultiply:   0x55,
	kvm.NumpadSubtract:   0x56,
	kvm.NumpadAdd:        0x57,
	kvm.NumpadEnter:      0x58,
	kvm.Numpad1:          0x59,
	kvm.Numpad2:          0x5A,
	kvm.Numpad3:          0x5B,
	kvm.Numpad4:          0x5C,
	kvm.Numpad5:          0x5D,
	kvm.Numpad6:          0x5E,
	kvm.Numpad7:          0x5F,
	kvm.Numpad8:          0x60,
	kvm.Numpad9:          0x61,
	kvm.Numpad0:          0x62,
	kvm.NumpadDecimal:    0x63,
	kvm.IntlBackslash:    0x64,
	kvm.ContextMenu:      0x65,
	kvm.Power:            0x66,
	kvm.NumpadEqual:      0x67,
	kvm.F13:              0x68,
	kvm.F14:              0x69,
	kvm.F15:              0x6A,
	kvm.F16:              0x6B,
	kvm.F17:              0x6C,
	kvm.F18:              0x6D,
	kvm.F19:              0x6E,
	kvm.F20:              0x6F,
	kvm.F21:              0x70,
	kvm.F22:              0x71,
	kvm.F23:              0x72,
	kvm.F24:              0x73,
	kvm.Help:             0x75,
	kvm.AudioVolumeMute:  0x7F,
	kvm.AudioVolumeUp:    0x80,
	kvm.AudioVolumeDown:  0x81,
	kvm.NumpadComma:      0x85,
	kvm.IntlRo:           0x87,
	kvm.KanaMode:         0x88,
	kvm.IntlYen:          0x89,
	kvm.Convert:          0x8A,
	kvm.NonConvert:       0x8B,
	kvm.Lang1:            0x90,
	kvm.Lang2:            0x91,
	kvm.Lang3:            0x92,
	kvm.Lang4:            0x93,
	kvm.Lang5:            0x94,
	kvm.NumpadParenLeft:  0xB6,
	kvm.NumpadParenRight: 0xB7,

	kvm.ControlLeft:  0xE0,
	kvm.ShiftLeft:    0xE1,
	kvm.AltLeft:      0xE2,
	kvm.MetaLeft:     0xE3,
	kvm.ControlRight: 0xE4,
	kvm.ShiftRight:   0xE5,
	kvm.AltRight:     0xE6,
	kvm.MetaRight:    0xE7,
}

// EncodeKeyEvent renders ke as the 3-byte serial frame [0xFF, mod, kc] the
// firmware expects. The keycode byte is 0 when the edge is on a modifier key,
// since the modifier state is conveyed via mod. Returns false if ke.Code has
// no HID mapping.
func EncodeKeyEvent(ke kvm.KeyEvent) ([3]byte, bool) {
	if ke.Code.IsModifier() {
		return [3]byte{syncByte, uint8(ke.Modifiers), 0}, true
	}
	kc, ok := hidUsage[ke.Code]
	if !ok {
		return [3]byte{}, false
	}
	if ke.Type == kvm.KeyTypeUp {
		kc = 0
	}
	return [3]byte{syncByte, uint8(ke.Modifiers), kc}, true
}

const syncByte = 0xFF
