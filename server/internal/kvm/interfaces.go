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

import "context"

// VideoSource is implemented by drivers that produce encoded video frames
// (v4l, synthetic, future codec-specific drivers).
type VideoSource interface {
	Shape() StreamShape
	InitData() []byte
	RequestKeyframe() error
	Subscribe(ctx context.Context) <-chan VideoFrame
}

// FrameSink consumes encoded video frames. Implemented by Client video
// transports; backpressure policy (drop vs. block) is the implementation's
// choice.
type FrameSink interface {
	WriteFrame(vf VideoFrame)
}

// KeyEventSink consumes abstract KeyEvents and translates them to the
// driver's wire protocol (USB HID for picolink). Implementations are
// stateless w.r.t. domain concerns; the Application owns KeyboardState.
type KeyEventSink interface {
	ReportKeyEvent(ke KeyEvent)
}

// MessageWriter is used by the Application to push outbound control messages
// to a Client. For browser clients this is bridged onto the WebSocket by
// http/websocket.Codec.
type MessageWriter interface {
	WriteMessage(msgType string, payload any) error
}
