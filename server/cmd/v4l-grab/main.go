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

// Command v4l-grab is a tiny operator helper for HydraKVM. It enumerates
// V4L2 capture devices (-list) or records a few seconds of MJPEG from one
// device into a file (-device + -out), exercising the same internal/v4l
// driver the server uses.
//
// Build target: linux/amd64 and linux/arm64. The enumeration mode hand-rolls
// the V4L2 VIDIOC_QUERYCAP and VIDIOC_ENUM_FMT ioctls.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sayotte/hydrakvm/internal/v4l"
)

func main() {
	listMode := flag.Bool("list", false, "enumerate /dev/video* devices and print their capabilities")
	device := flag.String("device", "", "V4L2 device path (e.g. /dev/video0) for capture mode")
	out := flag.String("out", "", "output file path for captured MJPEG stream")
	durSec := flag.Int("seconds", 5, "capture duration in seconds")
	width := flag.Int("width", 0, "capture width hint passed to ffmpeg (0 = device default)")
	height := flag.Int("height", 0, "capture height hint passed to ffmpeg (0 = device default)")
	framerate := flag.Int("framerate", 30, "framerate hint passed to ffmpeg")
	verbose := flag.Bool("verbose", false, "ffmpeg log level — when set, ffmpeg's stderr is logged verbosely; useful for debugging device/format negotiation")
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(w, "v4l-grab — HydraKVM V4L2 capture helper\n\nUsage:\n")
		_, _ = fmt.Fprintf(w, "  %s -list\n", filepath.Base(os.Args[0]))                                                                                 //nolint:gosec // CLI usage banner; not a web sink
		_, _ = fmt.Fprintf(w, "  %s -device /dev/videoN -out file.mjpg [-seconds 5] [-width W -height H] [-framerate N]\n\n", filepath.Base(os.Args[0])) //nolint:gosec // CLI usage banner; not a web sink
		_, _ = fmt.Fprintf(w, "Modes:\n  -list             Enumerate /dev/video* devices via V4L2 ioctls.\n")
		_, _ = fmt.Fprintf(w, "  -device + -out    Record MJPEG via internal/v4l for -seconds and write to -out.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *listMode || (*device == "" && *out == "") {
		if err := listDevices(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "v4l-grab:", err)
			os.Exit(1)
		}
		return
	}

	if *device == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "v4l-grab: -device and -out are both required for capture mode")
		flag.Usage()
		os.Exit(2)
	}
	if err := capture(*device, *out, *durSec, *width, *height, *framerate, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "v4l-grab:", err)
		os.Exit(1)
	}
}

func capture(device, outPath string, durSec, w, h, fr int, verbose bool) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logLevel := ""
	if verbose {
		logLevel = "info"
	}
	stream := v4l.New(v4l.Config{
		DevicePath:     device,
		Width:          w,
		Height:         h,
		Framerate:      fr,
		FFmpegLogLevel: logLevel,
	}, logger)
	defer stream.Close()

	f, err := os.Create(outPath) //nolint:gosec // outPath is operator-supplied CLI flag
	if err != nil {
		return fmt.Errorf("open output: %w", err)
	}
	defer func() { _ = f.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	deadline, deadlineCancel := context.WithTimeout(ctx, time.Duration(durSec)*time.Second)
	defer deadlineCancel()

	frames := stream.Subscribe(deadline)
	count := 0
	for {
		select {
		case <-deadline.Done():
			fmt.Fprintf(os.Stderr, "v4l-grab: wrote %d frames to %s\n", count, outPath)
			return nil
		case vf, ok := <-frames:
			if !ok {
				if count == 0 {
					return errors.New("source closed before any frame arrived")
				}
				fmt.Fprintf(os.Stderr, "v4l-grab: source closed after %d frames\n", count)
				return nil
			}
			if _, err := f.Write(vf.Data); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			count++
		}
	}
}

// listDevices enumerates /dev/video* and prints VIDIOC_QUERYCAP +
// VIDIOC_ENUM_FMT results. Linux only; on non-Linux builds the
// listDevicesPlatform stub returns an error.
func listDevices(w io.Writer) error {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return fmt.Errorf("glob /dev/video*: %w", err)
	}
	if len(matches) == 0 {
		_, _ = fmt.Fprintln(w, "no /dev/video* devices found")
		return nil
	}
	for _, dev := range matches {
		if err := describeDevice(w, dev); err != nil {
			_, _ = fmt.Fprintf(w, "%s: %v\n", dev, err)
			continue
		}
	}
	return nil
}
