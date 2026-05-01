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

package v4l

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

// fakeJPEG returns a minimal JPEG-shaped byte slice: SOI, payload, EOI.
// The payload is the supplied marker byte repeated, so frames can be
// distinguished in tests.
func fakeJPEG(marker byte, payloadLen int) []byte {
	out := make([]byte, 0, 4+payloadLen)
	out = append(out, 0xFF, 0xD8)
	for range payloadLen {
		out = append(out, marker)
	}
	out = append(out, 0xFF, 0xD9)
	return out
}

func TestJPEGSplitterMultipleFrames(t *testing.T) {
	a := fakeJPEG(0xAA, 7)
	b := fakeJPEG(0xBB, 11)
	c := fakeJPEG(0xCC, 3)
	stream := bytes.Join([][]byte{a, b, c}, nil)

	sp := newJPEGSplitter(bytes.NewReader(stream))
	for i, want := range [][]byte{a, b, c} {
		got, err := sp.next()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("frame %d:\ngot  %x\nwant %x", i, got, want)
		}
	}
	if _, err := sp.next(); !errors.Is(err, io.EOF) {
		t.Errorf("after last frame: got %v want EOF", err)
	}
}

func TestJPEGSplitterDropsGarbageBeforeSOI(t *testing.T) {
	a := fakeJPEG(0xAA, 5)
	stream := bytes.Join([][]byte{
		{0x00, 0x01, 0xFF, 0xFE, 0x77, 0xFF, 0x00},
		a,
	}, nil)

	sp := newJPEGSplitter(bytes.NewReader(stream))
	got, err := sp.next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !bytes.Equal(got, a) {
		t.Errorf("got %x want %x", got, a)
	}
}

func TestJPEGSplitterByteByByteReader(t *testing.T) {
	a := fakeJPEG(0xAA, 13)
	b := fakeJPEG(0xBB, 5)
	stream := bytes.Join([][]byte{a, b}, nil)

	sp := newJPEGSplitter(iotest.OneByteReader(bytes.NewReader(stream)))
	for i, want := range [][]byte{a, b} {
		got, err := sp.next()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("frame %d mismatch", i)
		}
	}
}

func TestJPEGSplitterTruncatedFrameIsEOF(t *testing.T) {
	stream := []byte{0xFF, 0xD8, 0x12, 0x34}
	sp := newJPEGSplitter(bytes.NewReader(stream))
	if _, err := sp.next(); !errors.Is(err, io.EOF) {
		t.Errorf("got %v want EOF", err)
	}
}
