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

//go:build linux && (amd64 || arm64)

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// V4L2 ioctl numbers. The encoding (_IOWR/_IOR with size of struct) is
// architecture-dependent only via pointer width, which on amd64/arm64 is the
// same; the constants below were verified against linux/videodev2.h headers
// for both. Keeping the constants literal avoids dragging in a build-time
// dependency on cgo or kernel headers.
const (
	vidiocQueryCap           = 0x80685600
	vidiocEnumFmt            = 0xc0405602
	vidiocEnumFramesizes     = 0xc02c564a
	vidiocEnumFrameintervals = 0xc034564b
	v4l2BufTypeVideoCapture  = 1

	v4l2FrmsizeTypeDiscrete   = 1
	v4l2FrmsizeTypeContinuous = 2
	v4l2FrmsizeTypeStepwise   = 3

	v4l2FrmivalTypeDiscrete   = 1
	v4l2FrmivalTypeContinuous = 2
	v4l2FrmivalTypeStepwise   = 3
)

// v4l2Capability mirrors struct v4l2_capability in linux/videodev2.h.
type v4l2Capability struct {
	Driver       [16]byte
	Card         [32]byte
	BusInfo      [32]byte
	Version      uint32
	Capabilities uint32
	DeviceCaps   uint32
	Reserved     [3]uint32
}

// v4l2Fmtdesc mirrors struct v4l2_fmtdesc.
type v4l2Fmtdesc struct {
	Index       uint32
	Type        uint32
	Flags       uint32
	Description [32]byte
	Pixelformat uint32
	Mbus_code   uint32 //nolint:revive,stylecheck // mirrors kernel struct field name
	Reserved    [3]uint32
}

// v4l2Frmsizeenum mirrors struct v4l2_frmsizeenum. The 24-byte payload
// after the header is a union of v4l2_frmsize_discrete (8 bytes) and
// v4l2_frmsize_stepwise (24 bytes); we read it as a fixed byte array and
// decode based on Type.
type v4l2Frmsizeenum struct {
	Index       uint32
	PixelFormat uint32
	Type        uint32
	Union       [24]byte
	Reserved    [2]uint32
}

// v4l2Frmivalenum mirrors struct v4l2_frmivalenum. The 24-byte payload
// after the header is a union of v4l2_fract (8 bytes) and a stepwise
// triple of v4l2_fract (24 bytes).
type v4l2Frmivalenum struct {
	Index       uint32
	PixelFormat uint32
	Width       uint32
	Height      uint32
	Type        uint32
	Union       [24]byte
	Reserved    [2]uint32
}

func describeDevice(w io.Writer, path string) error {
	f, err := os.OpenFile(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // path comes from filepath.Glob("/dev/video*")
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	var cap v4l2Capability
	if err := ioctl(f.Fd(), vidiocQueryCap, unsafe.Pointer(&cap)); err != nil {
		return fmt.Errorf("VIDIOC_QUERYCAP: %w", err)
	}
	fmt.Fprintf(w, "%s\n", path)
	fmt.Fprintf(w, "  driver:   %s\n", cstr(cap.Driver[:]))
	fmt.Fprintf(w, "  card:     %s\n", cstr(cap.Card[:]))
	fmt.Fprintf(w, "  bus_info: %s\n", cstr(cap.BusInfo[:]))
	fmt.Fprintf(w, "  caps:     %#08x  device_caps: %#08x\n", cap.Capabilities, cap.DeviceCaps)

	for i := range uint32(32) {
		desc := v4l2Fmtdesc{Index: i, Type: v4l2BufTypeVideoCapture}
		if err := ioctl(f.Fd(), vidiocEnumFmt, unsafe.Pointer(&desc)); err != nil {
			break
		}
		fmt.Fprintf(w, "  fmt[%d]:   %s   (%s)\n", i, fourCC(desc.Pixelformat), cstr(desc.Description[:]))
		describeFrameSizes(w, f.Fd(), desc.Pixelformat)
	}
	return nil
}

// describeFrameSizes enumerates frame sizes for a pixel format and, for
// each discrete size, enumerates frame intervals. CONTINUOUS and STEPWISE
// kinds are summarized rather than expanded; UVC devices report DISCRETE
// in practice.
func describeFrameSizes(w io.Writer, fd uintptr, pixfmt uint32) {
	for i := range uint32(64) {
		fs := v4l2Frmsizeenum{Index: i, PixelFormat: pixfmt}
		if err := ioctl(fd, vidiocEnumFramesizes, unsafe.Pointer(&fs)); err != nil {
			if !errors.Is(err, syscall.EINVAL) {
				fmt.Fprintf(w, "    (frame sizes unavailable: %v)\n", err)
			}
			return
		}
		switch fs.Type {
		case v4l2FrmsizeTypeDiscrete:
			width := readU32(fs.Union[:], 0)
			height := readU32(fs.Union[:], 4)
			fmt.Fprintf(w, "    %dx%d", width, height)
			describeFrameIntervalsDiscrete(w, fd, pixfmt, width, height)
			fmt.Fprintln(w)
		case v4l2FrmsizeTypeContinuous, v4l2FrmsizeTypeStepwise:
			minW := readU32(fs.Union[:], 0)
			maxW := readU32(fs.Union[:], 4)
			stepW := readU32(fs.Union[:], 8)
			minH := readU32(fs.Union[:], 12)
			maxH := readU32(fs.Union[:], 16)
			stepH := readU32(fs.Union[:], 20)
			label := "stepwise"
			if fs.Type == v4l2FrmsizeTypeContinuous {
				label = "continuous"
			}
			fmt.Fprintf(w, "    %s: w=%d..%d step %d, h=%d..%d step %d\n",
				label, minW, maxW, stepW, minH, maxH, stepH)
			return
		default:
			return
		}
	}
}

func describeFrameIntervalsDiscrete(w io.Writer, fd uintptr, pixfmt, width, height uint32) {
	first := true
	for i := range uint32(64) {
		fi := v4l2Frmivalenum{Index: i, PixelFormat: pixfmt, Width: width, Height: height}
		if err := ioctl(fd, vidiocEnumFrameintervals, unsafe.Pointer(&fi)); err != nil {
			if !errors.Is(err, syscall.EINVAL) {
				fmt.Fprintf(w, " (frame intervals unavailable: %v)", err)
			}
			return
		}
		switch fi.Type {
		case v4l2FrmivalTypeDiscrete:
			num := readU32(fi.Union[:], 0)
			den := readU32(fi.Union[:], 4)
			if first {
				fmt.Fprintf(w, " @ ")
				first = false
			} else {
				fmt.Fprintf(w, ", ")
			}
			fmt.Fprintf(w, "%s", formatFPS(num, den))
		case v4l2FrmivalTypeContinuous, v4l2FrmivalTypeStepwise:
			minN := readU32(fi.Union[:], 0)
			minD := readU32(fi.Union[:], 4)
			maxN := readU32(fi.Union[:], 8)
			maxD := readU32(fi.Union[:], 12)
			stepN := readU32(fi.Union[:], 16)
			stepD := readU32(fi.Union[:], 20)
			label := "stepwise"
			if fi.Type == v4l2FrmivalTypeContinuous {
				label = "continuous"
			}
			fmt.Fprintf(w, " @ %s: %s..%s step %s",
				label,
				formatFPS(minN, minD),
				formatFPS(maxN, maxD),
				formatFPS(stepN, stepD))
			return
		default:
			return
		}
	}
}

func readU32(b []byte, off int) uint32 {
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

// formatFPS converts a v4l2_fract (numerator/denominator seconds-per-frame)
// to a frames-per-second string rounded to one decimal.
func formatFPS(num, den uint32) string {
	if num == 0 {
		return "?"
	}
	fps := float64(den) / float64(num)
	return fmt.Sprintf("%.2f", fps)
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

func fourCC(v uint32) string {
	out := make([]byte, 4)
	for i := range 4 {
		c := byte((v >> (8 * i)) & 0xFF)
		if c < 0x20 || c > 0x7E {
			c = '?'
		}
		out[i] = c
	}
	return string(out)
}
