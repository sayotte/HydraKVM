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

// KeyCode is the canonical kvm-side identifier for a physical key, modeled on
// the W3C UI Events KeyboardEvent.code enumeration
// (https://www.w3.org/TR/uievents-code/). Numeric values are opaque; callers
// must use the named constants.
type KeyCode int

// KeyCode constants. KeyCodeReset is the zero value and acts as a sentinel
// meaning "no key" / "reset state"; it never corresponds to a real key.
const (
	KeyCodeReset KeyCode = iota

	Backquote
	Backslash
	BracketLeft
	BracketRight
	Comma
	Digit0
	Digit1
	Digit2
	Digit3
	Digit4
	Digit5
	Digit6
	Digit7
	Digit8
	Digit9
	Equal
	IntlBackslash
	IntlRo
	IntlYen
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Minus
	Period
	Quote
	Semicolon
	Slash

	AltLeft
	AltRight
	Backspace
	CapsLock
	ContextMenu
	ControlLeft
	ControlRight
	Enter
	MetaLeft
	MetaRight
	ShiftLeft
	ShiftRight
	Space
	Tab
	Convert
	KanaMode
	Lang1
	Lang2
	Lang3
	Lang4
	Lang5
	NonConvert

	Delete
	End
	Help
	Home
	Insert
	PageDown
	PageUp

	ArrowDown
	ArrowLeft
	ArrowRight
	ArrowUp

	NumLock
	Numpad0
	Numpad1
	Numpad2
	Numpad3
	Numpad4
	Numpad5
	Numpad6
	Numpad7
	Numpad8
	Numpad9
	NumpadAdd
	NumpadBackspace
	NumpadClear
	NumpadClearEntry
	NumpadComma
	NumpadDecimal
	NumpadDivide
	NumpadEnter
	NumpadEqual
	NumpadHash
	NumpadMemoryAdd
	NumpadMemoryClear
	NumpadMemoryRecall
	NumpadMemoryStore
	NumpadMemorySubtract
	NumpadMultiply
	NumpadParenLeft
	NumpadParenRight
	NumpadStar
	NumpadSubtract

	Escape
	F1
	F2
	F3
	F4
	F5
	F6
	F7
	F8
	F9
	F10
	F11
	F12
	F13
	F14
	F15
	F16
	F17
	F18
	F19
	F20
	F21
	F22
	F23
	F24
	Fn
	FnLock
	PrintScreen
	ScrollLock
	Pause

	BrowserBack
	BrowserFavorites
	BrowserForward
	BrowserHome
	BrowserRefresh
	BrowserSearch
	BrowserStop
	Eject
	LaunchApp1
	LaunchApp2
	LaunchMail
	MediaPlayPause
	MediaSelect
	MediaStop
	MediaTrackNext
	MediaTrackPrevious
	Power
	Sleep
	AudioVolumeDown
	AudioVolumeMute
	AudioVolumeUp
	WakeUp
)

// KeyType distinguishes a key-up edge from a key-down edge. KeyTypeUp is the
// zero value by deliberate choice: an unset KeyType decodes as "release",
// which is the safer default if a producer ever forgets to populate it.
type KeyType int

const (
	KeyTypeUp KeyType = iota
	KeyTypeDown
)

// KeyEventTranslator converts wire-side string identifiers (W3C
// KeyboardEvent.code values, edge names like "down"/"up") into the kvm-side
// KeyCode and KeyType enums.
//
// The interface is defined in package kvm rather than at its consumer because
// the natural consumer (cmd/hydrakvm wiring delegating to internal/dispatch)
// and the natural implementer (internal/http/websocket) both already import
// kvm. Placing the interface in either of those packages would force the
// other to import it, violating the dependency directions in
// docs/code-map.md. Hosting it in kvm keeps the contract on the package every
// participant already depends on.
type KeyEventTranslator interface {
	ParseKeyCode(s string) (KeyCode, bool)
	ParseKeyType(s string) (KeyType, bool)
}

func (c KeyCode) IsModifier() bool {
	switch c {
	case ControlLeft, ShiftLeft, AltLeft, MetaLeft,
		ControlRight, ShiftRight, AltRight, MetaRight:
		return true
	}
	return false
}

func (c KeyCode) ModifierBit() ModifierMask {
	switch c {
	case ControlLeft:
		return ModLeftCtrl
	case ShiftLeft:
		return ModLeftShift
	case AltLeft:
		return ModLeftAlt
	case MetaLeft:
		return ModLeftMeta
	case ControlRight:
		return ModRightCtrl
	case ShiftRight:
		return ModRightShift
	case AltRight:
		return ModRightAlt
	case MetaRight:
		return ModRightMeta
	}
	return 0
}
