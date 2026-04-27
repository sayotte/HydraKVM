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
	"testing"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

func TestVideoSourceSatisfiesInterface(t *testing.T) {
	var _ kvm.VideoSource = NewVideoSource()
}

func TestShapeAdvertisesMJPEG(t *testing.T) {
	s := NewVideoSource().Shape()
	if s.Codec != "mjpeg" || s.MIMEType != "image/jpeg" || s.Framing != "multipart" {
		t.Errorf("unexpected shape: %+v", s)
	}
}

func TestSubscribeClosesOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := NewVideoSource().Subscribe(ctx)
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("stub source must not emit frames")
		}
	case <-time.After(time.Second):
		t.Fatal("subscribe channel did not close after cancel")
	}
}
