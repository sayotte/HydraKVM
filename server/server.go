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

package main

import (
	"fmt"
	"os"

	"go.bug.st/serial"
	"golang.org/x/term"
)

type hidKey struct {
	mod byte
	kc  byte
}

const shift = 0x02

var asciiToHID = map[byte]hidKey{
	'a': {0, 0x04}, 'b': {0, 0x05}, 'c': {0, 0x06}, 'd': {0, 0x07},
	'e': {0, 0x08}, 'f': {0, 0x09}, 'g': {0, 0x0A}, 'h': {0, 0x0B},
	'i': {0, 0x0C}, 'j': {0, 0x0D}, 'k': {0, 0x0E}, 'l': {0, 0x0F},
	'm': {0, 0x10}, 'n': {0, 0x11}, 'o': {0, 0x12}, 'p': {0, 0x13},
	'q': {0, 0x14}, 'r': {0, 0x15}, 's': {0, 0x16}, 't': {0, 0x17},
	'u': {0, 0x18}, 'v': {0, 0x19}, 'w': {0, 0x1A}, 'x': {0, 0x1B},
	'y': {0, 0x1C}, 'z': {0, 0x1D},

	'A': {shift, 0x04}, 'B': {shift, 0x05}, 'C': {shift, 0x06}, 'D': {shift, 0x07},
	'E': {shift, 0x08}, 'F': {shift, 0x09}, 'G': {shift, 0x0A}, 'H': {shift, 0x0B},
	'I': {shift, 0x0C}, 'J': {shift, 0x0D}, 'K': {shift, 0x0E}, 'L': {shift, 0x0F},
	'M': {shift, 0x10}, 'N': {shift, 0x11}, 'O': {shift, 0x12}, 'P': {shift, 0x13},
	'Q': {shift, 0x14}, 'R': {shift, 0x15}, 'S': {shift, 0x16}, 'T': {shift, 0x17},
	'U': {shift, 0x18}, 'V': {shift, 0x19}, 'W': {shift, 0x1A}, 'X': {shift, 0x1B},
	'Y': {shift, 0x1C}, 'Z': {shift, 0x1D},

	'1': {0, 0x1E}, '2': {0, 0x1F}, '3': {0, 0x20}, '4': {0, 0x21},
	'5': {0, 0x22}, '6': {0, 0x23}, '7': {0, 0x24}, '8': {0, 0x25},
	'9': {0, 0x26}, '0': {0, 0x27},

	'!': {shift, 0x1E}, '@': {shift, 0x1F}, '#': {shift, 0x20}, '$': {shift, 0x21},
	'%': {shift, 0x22}, '^': {shift, 0x23}, '&': {shift, 0x24}, '*': {shift, 0x25},
	'(': {shift, 0x26}, ')': {shift, 0x27},

	' ':  {0, 0x2C},
	'\n': {0, 0x28},
	'\r': {0, 0x28},
	'\t': {0, 0x2B},
	'\b': {0, 0x2A},
	0x7F: {0, 0x2A}, // macOS sends DEL for backspace
	',':  {0, 0x36},
	'.':  {0, 0x37},
	'/':  {0, 0x38},
	';':  {0, 0x33},
	'\'': {0, 0x34},
	'[':  {0, 0x2F},
	']':  {0, 0x30},
	'\\': {0, 0x31},
	'-':  {0, 0x2D},
	'=':  {0, 0x2E},
	'`':  {0, 0x35},

	'<': {shift, 0x36}, '>': {shift, 0x37}, '?': {shift, 0x38},
	':': {shift, 0x33}, '"': {shift, 0x34}, '{': {shift, 0x2F},
	'}': {shift, 0x30}, '|': {shift, 0x31}, '_': {shift, 0x2D},
	'+': {shift, 0x2E}, '~': {shift, 0x35},
}

// Ctrl+letter: terminal sends 0x01-0x1A
var ctrlToHID = map[byte]hidKey{
	0x01: {0x01, 0x04}, // Ctrl+A
	0x02: {0x01, 0x05}, // Ctrl+B
	0x03: {0x01, 0x06}, // Ctrl+C
	0x04: {0x01, 0x07}, // Ctrl+D
	0x05: {0x01, 0x08}, // Ctrl+E
	0x06: {0x01, 0x09}, // Ctrl+F
	0x07: {0x01, 0x0A}, // Ctrl+G
	0x08: {0x01, 0x0B}, // Ctrl+H (also backspace on some terminals)
	0x09: {0x01, 0x0C}, // Ctrl+I (also tab)
	0x0A: {0x01, 0x0D}, // Ctrl+J (also linefeed)
	0x0B: {0x01, 0x0E}, // Ctrl+K
	0x0C: {0x01, 0x0F}, // Ctrl+L
	0x0D: {0x01, 0x10}, // Ctrl+M (also carriage return)
	0x0E: {0x01, 0x11}, // Ctrl+N
	0x0F: {0x01, 0x12}, // Ctrl+O
	0x10: {0x01, 0x13}, // Ctrl+P
	0x11: {0x01, 0x14}, // Ctrl+Q
	0x12: {0x01, 0x15}, // Ctrl+R
	0x13: {0x01, 0x16}, // Ctrl+S
	0x14: {0x01, 0x17}, // Ctrl+T
	0x15: {0x01, 0x18}, // Ctrl+U
	0x16: {0x01, 0x19}, // Ctrl+V
	0x17: {0x01, 0x1A}, // Ctrl+W
	0x18: {0x01, 0x1B}, // Ctrl+X
	0x19: {0x01, 0x1C}, // Ctrl+Y
	0x1A: {0x01, 0x1D}, // Ctrl+Z
	0x1B: {0, 0x29},    // Escape
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s /dev/cu.usbserialXXXX\n", os.Args[0])
		os.Exit(1)
	}

	port, err := serial.Open(os.Args[1], &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	defer port.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Fprintf(os.Stderr, "Connected. Ctrl+] to quit.\r\n")

	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		ch := buf[0]

		// Ctrl+] to exit
		if ch == 0x1D {
			break
		}

		// Check ASCII table first
		if k, ok := asciiToHID[ch]; ok {
			port.Write([]byte{0xFF, k.mod, k.kc})
			continue
		}

		// Check control characters
		if k, ok := ctrlToHID[ch]; ok {
			port.Write([]byte{0xFF, k.mod, k.kc})
			continue
		}
	}
}
