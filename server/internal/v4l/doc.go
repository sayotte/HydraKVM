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

// Package v4l implements [kvm.VideoSource] for V4L2 capture devices by
// shelling out to ffmpeg and parsing its MJPEG stdout. The driver is
// stateless w.r.t. domain decisions: it surfaces "source up" and "source
// down" through the [kvm.VideoSource.Subscribe] channel lifecycle (open
// while frames are flowing; closed while ctx is still live to signal
// failure) and lets [kvm.Application] decide what to do about it.
package v4l
