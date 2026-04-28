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
	"image/jpeg"
	"sync"
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

func TestSatisfiesVideoSourceInterface(t *testing.T) {
	var _ kvm.VideoSource = (*VideoSource)(nil)
}

func TestShapeAdvertisesMJPEG(t *testing.T) {
	s := NewVideoSource("x").Shape()
	if s.Codec != "mjpeg" || s.MIMEType != "image/jpeg" || s.Framing != "multipart" {
		t.Errorf("unexpected shape: %+v", s)
	}
	if s.Width != defaultWidth || s.Height != defaultHeight || s.Framerate != defaultFramerate {
		t.Errorf("unexpected dims: %+v", s)
	}
}

func TestSubscribeReceivesFrames(t *testing.T) {
	src := NewVideoSource("test")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := src.Subscribe(ctx)
	got := 0
	deadline := time.After(2 * time.Second)
	for got < 2 {
		select {
		case f, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed early after %d frames", got)
			}
			if !f.IsKey {
				t.Error("frame not marked as keyframe")
			}
			if len(f.Data) < 3 || f.Data[0] != 0xFF || f.Data[1] != 0xD8 || f.Data[2] != 0xFF {
				t.Errorf("frame %d: not a JPEG SOI", got)
			}
			got++
		case <-deadline:
			t.Fatalf("only got %d frames in 2s", got)
		}
	}
}

func TestSubscribeUnregistersOnContextCancel(t *testing.T) {
	src := NewVideoSource("x")
	ctx, cancel := context.WithCancel(context.Background())
	ch := src.Subscribe(ctx)
	cancel()

	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				goto closed
			}
		case <-deadline:
			t.Fatal("channel did not close after cancel")
		}
	}
closed:
	time.Sleep(300 * time.Millisecond)
	src.mu.Lock()
	n := len(src.subscribers)
	src.mu.Unlock()
	if n != 0 {
		t.Errorf("subscribers map has %d entries; want 0", n)
	}
}

func TestSubscribeMultipleConcurrent(t *testing.T) {
	src := NewVideoSource("multi")
	ctxs := make([]context.CancelFunc, 3)
	chans := make([]<-chan kvm.VideoFrame, 3)
	for i := range 3 {
		ctx, cancel := context.WithCancel(context.Background())
		ctxs[i] = cancel
		chans[i] = src.Subscribe(ctx)
	}

	var wg sync.WaitGroup
	for i, c := range chans {
		wg.Add(1)
		go func(i int, c <-chan kvm.VideoFrame) {
			defer wg.Done()
			select {
			case f, ok := <-c:
				if !ok {
					t.Errorf("subscriber %d closed before first frame", i)
					return
				}
				if len(f.Data) == 0 {
					t.Errorf("subscriber %d empty frame", i)
				}
			case <-time.After(2 * time.Second):
				t.Errorf("subscriber %d got no frame", i)
			}
		}(i, c)
	}
	wg.Wait()

	for _, c := range ctxs {
		c()
	}
	time.Sleep(300 * time.Millisecond)

	src.mu.Lock()
	n := len(src.subscribers)
	src.mu.Unlock()
	if n != 0 {
		t.Errorf("subscribers map has %d entries after all cancels; want 0", n)
	}
}

func TestRenderFrameContainsBackground(t *testing.T) {
	src := NewVideoSource("bg")
	data, err := src.renderFrame(time.Unix(0, 0))
	if err != nil {
		t.Fatalf("renderFrame: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	b := img.Bounds()
	checkApproxBG := func(x, y int) {
		t.Helper()
		r, g, bl, _ := img.At(x, y).RGBA()
		r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(bl>>8) //nolint:gosec // RGBA() returns alpha-premultiplied 16-bit values; the high byte is always within uint8 range
		if absDiff(r8, 0x1c) > 16 || absDiff(g8, 0x1a) > 16 || absDiff(b8, 0x18) > 16 {
			t.Errorf("pixel (%d,%d) = %02x%02x%02x; want approx 1c1a18", x, y, r8, g8, b8)
		}
	}
	checkApproxBG(b.Min.X+2, b.Min.Y+2)
	checkApproxBG(b.Max.X-3, b.Min.Y+2)
	checkApproxBG(b.Min.X+2, b.Max.Y-3)
	checkApproxBG(b.Max.X-3, b.Max.Y-3)
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
