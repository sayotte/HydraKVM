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

package synthetic

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

const (
	defaultWidth     = 640
	defaultHeight    = 360
	defaultFramerate = 5
	pixelScale       = 4
	jpegQuality      = 70
)

var (
	bgColor = color.RGBA{R: 0x1c, G: 0x1a, B: 0x18, A: 0xff}
	fgColor = color.RGBA{R: 0xa0, G: 0x70, B: 0x5a, A: 0xff}
)

// VideoSource is a [kvm.VideoSource] that emits a synthetic MJPEG feed: a
// solid-colored background with a configurable label and a wall-clock
// timestamp rendered using a hand-rolled bitmap font. Every frame is encoded
// as a fresh JPEG.
type VideoSource struct {
	label     string
	width     int
	height    int
	framerate int

	mu          sync.Mutex
	subscribers map[chan kvm.VideoFrame]struct{}
	started     atomic.Bool
	startedAt   time.Time
}

// NewVideoSource constructs a VideoSource with the given label drawn onto each
// frame; remaining parameters use compiled-in defaults (640x360 @ 5 fps).
func NewVideoSource(label string) *VideoSource {
	return &VideoSource{
		label:       label,
		width:       defaultWidth,
		height:      defaultHeight,
		framerate:   defaultFramerate,
		subscribers: make(map[chan kvm.VideoFrame]struct{}),
	}
}

// Shape reports the MJPEG stream parameters.
func (s *VideoSource) Shape() kvm.StreamShape {
	return kvm.StreamShape{
		Codec:     "mjpeg",
		MIMEType:  "image/jpeg",
		Framing:   "multipart",
		Width:     s.width,
		Height:    s.height,
		Framerate: s.framerate,
	}
}

// InitData returns nil; MJPEG has no codec init blob.
func (s *VideoSource) InitData() []byte { return nil }

// RequestKeyframe is a no-op for MJPEG (every frame is a keyframe).
func (s *VideoSource) RequestKeyframe() error { return nil }

// Subscribe registers a new subscriber and lazily starts the generator
// goroutine on the first call. The returned channel is closed when ctx is
// cancelled. Lazy start avoids burning CPU encoding frames when no client is
// watching.
func (s *VideoSource) Subscribe(ctx context.Context) <-chan kvm.VideoFrame {
	ch := make(chan kvm.VideoFrame, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	if s.started.CompareAndSwap(false, true) {
		s.startedAt = time.Now()
		go s.runGenerator()
	}

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}()

	return ch
}

func (s *VideoSource) runGenerator() {
	t := time.NewTicker(time.Second / time.Duration(s.framerate))
	defer t.Stop()
	for now := range t.C {
		data, err := s.renderFrame(now)
		if err != nil {
			continue
		}
		frame := kvm.VideoFrame{
			Data:  data,
			IsKey: true,
			PTS:   now.Sub(s.startedAt),
		}
		s.mu.Lock()
		for ch := range s.subscribers {
			select {
			case ch <- frame:
			default:
			}
		}
		s.mu.Unlock()
	}
}

func (s *VideoSource) renderFrame(now time.Time) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	fillRect(img, img.Bounds(), bgColor)

	line1 := s.label
	line2 := now.Format("15:04:05")

	cellW := glyphWidth * pixelScale
	cellH := glyphHeight * pixelScale
	gap := pixelScale

	w1 := len(line1)*(cellW+gap) - gap
	w2 := len(line2)*(cellW+gap) - gap

	x1 := (s.width - w1) / 2
	y1 := s.height/4 - cellH/2
	drawString(img, x1, y1, line1, fgColor)

	x2 := (s.width - w2) / 2
	y2 := s.height/2 - cellH/2
	drawString(img, x2, y2, line2, fgColor)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawString(img *image.RGBA, x, y int, s string, c color.RGBA) {
	cellW := glyphWidth * pixelScale
	gap := pixelScale
	for i := range len(s) {
		drawGlyph(img, x+i*(cellW+gap), y, s[i], c)
	}
}

func drawGlyph(img *image.RGBA, x, y int, ch byte, c color.RGBA) {
	g, ok := glyphs[ch]
	if !ok {
		return
	}
	for row := range glyphHeight {
		bits := g[row]
		for col := range glyphWidth {
			if bits&(1<<(glyphWidth-1-col)) == 0 {
				continue
			}
			px := x + col*pixelScale
			py := y + row*pixelScale
			fillRect(img, image.Rect(px, py, px+pixelScale, py+pixelScale), c)
		}
	}
}
