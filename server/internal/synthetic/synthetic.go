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
	"context"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

// VideoSource is a stub [kvm.VideoSource] that never emits frames. The
// returned subscription channel closes when the supplied context is
// cancelled. A real frame generator lands in Step 3.
type VideoSource struct{}

// NewVideoSource constructs a stub VideoSource.
func NewVideoSource() *VideoSource { return &VideoSource{} }

// Shape reports the eventual MJPEG shape so wiring downstream of this source
// can be exercised before any frames flow.
func (s *VideoSource) Shape() kvm.StreamShape {
	return kvm.StreamShape{
		Codec:    "mjpeg",
		MIMEType: "image/jpeg",
		Framing:  "multipart",
	}
}

// InitData returns nil; MJPEG has no codec init blob.
func (s *VideoSource) InitData() []byte { return nil }

// RequestKeyframe is a no-op for MJPEG.
func (s *VideoSource) RequestKeyframe() error { return nil }

// Subscribe returns a channel that is closed when ctx is cancelled and never
// produces a frame in the meantime.
func (s *VideoSource) Subscribe(ctx context.Context) <-chan kvm.VideoFrame {
	ch := make(chan kvm.VideoFrame)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch
}
