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

package picolink

import (
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/unix"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

// Keyboard implements [kvm.KeyEventSink] over a serial link to a Pico running
// the HydraKVM keyboard firmware. Each [kvm.KeyEvent] becomes a single 3-byte
// frame [0xFF, mod, kc]; the firmware applies it as a USB HID boot-keyboard
// report state snapshot.
type Keyboard struct {
	devicePath string
	f          *os.File
	logger     *slog.Logger
}

// NewKeyboard opens devicePath as a raw 115200 8N1 serial port and returns a
// keyboard sink writing into it. If logger is nil, [slog.Default] is used.
func NewKeyboard(devicePath string, logger *slog.Logger) (*Keyboard, error) {
	if logger == nil {
		logger = slog.Default()
	}
	f, err := os.OpenFile(devicePath, os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0) //nolint:gosec // devicePath is operator-supplied config, not user input
	if err != nil {
		return nil, fmt.Errorf("picolink: open %q: %w", devicePath, err)
	}
	fd := int(f.Fd()) //nolint:gosec // POSIX fd values fit in int
	if err := configureSerial(fd); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("picolink: configure %q: %w", devicePath, err)
	}
	// Drop O_NONBLOCK now that termios is configured; writes should block to
	// preserve frame ordering with the firmware's UART receiver.
	if err := unix.SetNonblock(fd, false); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("picolink: clear nonblock %q: %w", devicePath, err)
	}
	logger.Info("picolink opened", "device", devicePath)
	return &Keyboard{devicePath: devicePath, f: f, logger: logger}, nil
}

// Close releases the underlying serial port.
func (k *Keyboard) Close() error {
	if k.f == nil {
		return nil
	}
	err := k.f.Close()
	k.f = nil
	k.logger.Info("picolink closed", "device", k.devicePath)
	return err
}

// ReportKeyEvent encodes ke as a 3-byte serial frame and writes it. Encoding
// and write failures are logged; this sink is fire-and-forget per the
// [kvm.KeyEventSink] contract.
func (k *Keyboard) ReportKeyEvent(ke kvm.KeyEvent) {
	frame, ok := EncodeKeyEvent(ke)
	if !ok {
		k.logger.Debug("picolink: dropping unmappable key event",
			"code", ke.Code, "type", ke.Type)
		return
	}
	if _, err := k.f.Write(frame[:]); err != nil {
		k.logger.Error("picolink: serial write failed",
			"device", k.devicePath, "err", err)
	}
}

func configureSerial(fd int) error {
	t, err := unix.IoctlGetTermios(fd, getTermios)
	if err != nil {
		return fmt.Errorf("get termios: %w", err)
	}

	// Raw mode: clear all input/output/local processing; 8N1; receiver enabled;
	// ignore modem control lines.
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CSTOPB | unix.CRTSCTS
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0

	setBaud(t, unix.B115200)

	if err := unix.IoctlSetTermios(fd, setTermios, t); err != nil {
		return fmt.Errorf("set termios: %w", err)
	}
	return nil
}
