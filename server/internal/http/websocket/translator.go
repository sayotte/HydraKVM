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

import "github.com/sayotte/hydrakvm/internal/kvm"

// W3CKeyEventTranslator implements [kvm.KeyEventTranslator] for the W3C UI
// Events KeyboardEvent.code/type wire strings produced by browsers.
type W3CKeyEventTranslator struct{}

// NewW3CKeyEventTranslator returns a new W3CKeyEventTranslator. The translator
// is stateless; the constructor exists for symmetry with the rest of the
// package's API.
func NewW3CKeyEventTranslator() *W3CKeyEventTranslator { return &W3CKeyEventTranslator{} }

var _ kvm.KeyEventTranslator = (*W3CKeyEventTranslator)(nil)

// ParseKeyCode resolves a W3C KeyboardEvent.code string to its
// [kvm.KeyCode]. Unknown strings return ([kvm.KeyCodeReset], false).
func (W3CKeyEventTranslator) ParseKeyCode(s string) (kvm.KeyCode, bool) {
	c, ok := w3cCodeToKVM[s]
	if !ok {
		return kvm.KeyCodeReset, false
	}
	return c, true
}

// ParseKeyType resolves a W3C KeyboardEvent.type string ("keydown" / "keyup")
// to its [kvm.KeyType]. Unknown strings return ([kvm.KeyTypeUp], false).
func (W3CKeyEventTranslator) ParseKeyType(s string) (kvm.KeyType, bool) {
	switch s {
	case "keydown":
		return kvm.KeyTypeDown, true
	case "keyup":
		return kvm.KeyTypeUp, true
	default:
		return kvm.KeyTypeUp, false
	}
}

var w3cCodeToKVM = map[string]kvm.KeyCode{
	"Backquote":     kvm.Backquote,
	"Backslash":     kvm.Backslash,
	"BracketLeft":   kvm.BracketLeft,
	"BracketRight":  kvm.BracketRight,
	"Comma":         kvm.Comma,
	"Digit0":        kvm.Digit0,
	"Digit1":        kvm.Digit1,
	"Digit2":        kvm.Digit2,
	"Digit3":        kvm.Digit3,
	"Digit4":        kvm.Digit4,
	"Digit5":        kvm.Digit5,
	"Digit6":        kvm.Digit6,
	"Digit7":        kvm.Digit7,
	"Digit8":        kvm.Digit8,
	"Digit9":        kvm.Digit9,
	"Equal":         kvm.Equal,
	"IntlBackslash": kvm.IntlBackslash,
	"IntlRo":        kvm.IntlRo,
	"IntlYen":       kvm.IntlYen,
	"KeyA":          kvm.KeyA,
	"KeyB":          kvm.KeyB,
	"KeyC":          kvm.KeyC,
	"KeyD":          kvm.KeyD,
	"KeyE":          kvm.KeyE,
	"KeyF":          kvm.KeyF,
	"KeyG":          kvm.KeyG,
	"KeyH":          kvm.KeyH,
	"KeyI":          kvm.KeyI,
	"KeyJ":          kvm.KeyJ,
	"KeyK":          kvm.KeyK,
	"KeyL":          kvm.KeyL,
	"KeyM":          kvm.KeyM,
	"KeyN":          kvm.KeyN,
	"KeyO":          kvm.KeyO,
	"KeyP":          kvm.KeyP,
	"KeyQ":          kvm.KeyQ,
	"KeyR":          kvm.KeyR,
	"KeyS":          kvm.KeyS,
	"KeyT":          kvm.KeyT,
	"KeyU":          kvm.KeyU,
	"KeyV":          kvm.KeyV,
	"KeyW":          kvm.KeyW,
	"KeyX":          kvm.KeyX,
	"KeyY":          kvm.KeyY,
	"KeyZ":          kvm.KeyZ,
	"Minus":         kvm.Minus,
	"Period":        kvm.Period,
	"Quote":         kvm.Quote,
	"Semicolon":     kvm.Semicolon,
	"Slash":         kvm.Slash,

	"AltLeft":      kvm.AltLeft,
	"AltRight":     kvm.AltRight,
	"Backspace":    kvm.Backspace,
	"CapsLock":     kvm.CapsLock,
	"ContextMenu":  kvm.ContextMenu,
	"ControlLeft":  kvm.ControlLeft,
	"ControlRight": kvm.ControlRight,
	"Enter":        kvm.Enter,
	"MetaLeft":     kvm.MetaLeft,
	"MetaRight":    kvm.MetaRight,
	"ShiftLeft":    kvm.ShiftLeft,
	"ShiftRight":   kvm.ShiftRight,
	"Space":        kvm.Space,
	"Tab":          kvm.Tab,
	"Convert":      kvm.Convert,
	"KanaMode":     kvm.KanaMode,
	"Lang1":        kvm.Lang1,
	"Lang2":        kvm.Lang2,
	"Lang3":        kvm.Lang3,
	"Lang4":        kvm.Lang4,
	"Lang5":        kvm.Lang5,
	"NonConvert":   kvm.NonConvert,

	"Delete":   kvm.Delete,
	"End":      kvm.End,
	"Help":     kvm.Help,
	"Home":     kvm.Home,
	"Insert":   kvm.Insert,
	"PageDown": kvm.PageDown,
	"PageUp":   kvm.PageUp,

	"ArrowDown":  kvm.ArrowDown,
	"ArrowLeft":  kvm.ArrowLeft,
	"ArrowRight": kvm.ArrowRight,
	"ArrowUp":    kvm.ArrowUp,

	"NumLock":              kvm.NumLock,
	"Numpad0":              kvm.Numpad0,
	"Numpad1":              kvm.Numpad1,
	"Numpad2":              kvm.Numpad2,
	"Numpad3":              kvm.Numpad3,
	"Numpad4":              kvm.Numpad4,
	"Numpad5":              kvm.Numpad5,
	"Numpad6":              kvm.Numpad6,
	"Numpad7":              kvm.Numpad7,
	"Numpad8":              kvm.Numpad8,
	"Numpad9":              kvm.Numpad9,
	"NumpadAdd":            kvm.NumpadAdd,
	"NumpadBackspace":      kvm.NumpadBackspace,
	"NumpadClear":          kvm.NumpadClear,
	"NumpadClearEntry":     kvm.NumpadClearEntry,
	"NumpadComma":          kvm.NumpadComma,
	"NumpadDecimal":        kvm.NumpadDecimal,
	"NumpadDivide":         kvm.NumpadDivide,
	"NumpadEnter":          kvm.NumpadEnter,
	"NumpadEqual":          kvm.NumpadEqual,
	"NumpadHash":           kvm.NumpadHash,
	"NumpadMemoryAdd":      kvm.NumpadMemoryAdd,
	"NumpadMemoryClear":    kvm.NumpadMemoryClear,
	"NumpadMemoryRecall":   kvm.NumpadMemoryRecall,
	"NumpadMemoryStore":    kvm.NumpadMemoryStore,
	"NumpadMemorySubtract": kvm.NumpadMemorySubtract,
	"NumpadMultiply":       kvm.NumpadMultiply,
	"NumpadParenLeft":      kvm.NumpadParenLeft,
	"NumpadParenRight":     kvm.NumpadParenRight,
	"NumpadStar":           kvm.NumpadStar,
	"NumpadSubtract":       kvm.NumpadSubtract,

	"Escape":      kvm.Escape,
	"F1":          kvm.F1,
	"F2":          kvm.F2,
	"F3":          kvm.F3,
	"F4":          kvm.F4,
	"F5":          kvm.F5,
	"F6":          kvm.F6,
	"F7":          kvm.F7,
	"F8":          kvm.F8,
	"F9":          kvm.F9,
	"F10":         kvm.F10,
	"F11":         kvm.F11,
	"F12":         kvm.F12,
	"F13":         kvm.F13,
	"F14":         kvm.F14,
	"F15":         kvm.F15,
	"F16":         kvm.F16,
	"F17":         kvm.F17,
	"F18":         kvm.F18,
	"F19":         kvm.F19,
	"F20":         kvm.F20,
	"F21":         kvm.F21,
	"F22":         kvm.F22,
	"F23":         kvm.F23,
	"F24":         kvm.F24,
	"Fn":          kvm.Fn,
	"FnLock":      kvm.FnLock,
	"PrintScreen": kvm.PrintScreen,
	"ScrollLock":  kvm.ScrollLock,
	"Pause":       kvm.Pause,

	"BrowserBack":        kvm.BrowserBack,
	"BrowserFavorites":   kvm.BrowserFavorites,
	"BrowserForward":     kvm.BrowserForward,
	"BrowserHome":        kvm.BrowserHome,
	"BrowserRefresh":     kvm.BrowserRefresh,
	"BrowserSearch":      kvm.BrowserSearch,
	"BrowserStop":        kvm.BrowserStop,
	"Eject":              kvm.Eject,
	"LaunchApp1":         kvm.LaunchApp1,
	"LaunchApp2":         kvm.LaunchApp2,
	"LaunchMail":         kvm.LaunchMail,
	"MediaPlayPause":     kvm.MediaPlayPause,
	"MediaSelect":        kvm.MediaSelect,
	"MediaStop":          kvm.MediaStop,
	"MediaTrackNext":     kvm.MediaTrackNext,
	"MediaTrackPrevious": kvm.MediaTrackPrevious,
	"Power":              kvm.Power,
	"Sleep":              kvm.Sleep,
	"AudioVolumeDown":    kvm.AudioVolumeDown,
	"AudioVolumeMute":    kvm.AudioVolumeMute,
	"AudioVolumeUp":      kvm.AudioVolumeUp,
	"WakeUp":             kvm.WakeUp,
}
