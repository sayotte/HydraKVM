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
	"bufio"
	"io"
)

// jpegSplitter walks an MJPEG-over-pipe byte stream and yields one complete
// JPEG per call to next. It is robust to garbage between frames (anything
// before the next SOI is dropped) and to reads that split across SOI/EOI
// markers. The implementation does not look up image metadata; it only
// brackets bytes between SOI (0xFF 0xD8) and EOI (0xFF 0xD9).
type jpegSplitter struct {
	r *bufio.Reader
}

func newJPEGSplitter(r io.Reader) *jpegSplitter {
	return &jpegSplitter{r: bufio.NewReaderSize(r, 1<<16)}
}

// next reads bytes until a complete JPEG (SOI .. EOI) has been assembled,
// returning a fresh slice owned by the caller. Returns [io.EOF] (or another
// error) if the underlying reader fails before a frame can be completed.
func (j *jpegSplitter) next() ([]byte, error) {
	if err := j.skipToSOI(); err != nil {
		return nil, err
	}
	// SOI consumed by skipToSOI. Re-emit it as the first two bytes of the
	// frame and read the rest until we see EOI.
	frame := make([]byte, 0, 64*1024)
	frame = append(frame, 0xFF, 0xD8)
	for {
		b, err := j.r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0xFF {
			frame = append(frame, b)
			continue
		}
		// 0xFF — peek at the next byte to classify.
		next, err := j.r.ReadByte()
		if err != nil {
			return nil, err
		}
		frame = append(frame, 0xFF, next)
		if next == 0xD9 {
			return frame, nil
		}
	}
}

// skipToSOI advances the reader past any prefix bytes until the SOI marker
// (0xFF 0xD8) has been consumed. Both bytes of the SOI are eaten before
// returning so the caller can resume reading the frame body.
func (j *jpegSplitter) skipToSOI() error {
	for {
		b, err := j.r.ReadByte()
		if err != nil {
			return err
		}
		if b != 0xFF {
			continue
		}
		next, err := j.r.ReadByte()
		if err != nil {
			return err
		}
		if next == 0xD8 {
			return nil
		}
		// Not SOI. The 0xFF we just read might actually be the first byte
		// of a real SOI two-byte sequence if next was itself 0xFF (a fill
		// byte). Push the second byte back into the loop's consideration
		// by treating it as the new candidate.
		if next == 0xFF {
			if err := j.r.UnreadByte(); err != nil {
				return err
			}
		}
	}
}
