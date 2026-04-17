# KVM

A low-cost, network-accessible KVM (Keyboard, Video, Monitor) built from commodity hardware. Provides remote keyboard input and video output for headless Linux and OpenBSD machines over HTTPS, targeting machines that refuse to boot or accept input without a physical keyboard and monitor connected.

## Architecture

```
                        HTTPS
  Browser  <─────────────────────────>  KVM Server (Linux)
  (keyboard input,                      (Go, serves web UI,
   MJPEG video)                          HDMI capture, UART control)
                                              │
                                              │ UART/TTL (115200 baud)
                                              │ or RS485 over Cat5e
                                              │
                                         Pico (RP2040)
                                              │
                                              │ USB (HID Keyboard)
                                              │
                                        Target Machine
                                              │
                                              │ HDMI
                                              │
                                        Capture Dongle ──── USB ──── KVM Server
```

### Keyboard path

The KVM server sends 2-byte HID-encoded keystroke pairs (`{modifier, keycode}`) over a serial connection to a Raspberry Pi Pico. The Pico presents itself to the target machine as a standard USB HID keyboard and replays each pair as a keypress-release cycle. The target machine sees a normal keyboard — no drivers, no software, works from BIOS/bootloader onwards.

### Video path

A MACROSILICON-based USB HDMI capture dongle receives the target machine's HDMI output and exposes it as a UVC device. The KVM server reads MJPEG frames from the dongle via V4L2 and serves them to the browser. The dongle handles scaling and JPEG compression internally.

### Wire protocol

Every message from the KVM server to the Pico is exactly 2 bytes: one modifier bitmask and one HID usage ID keycode. No framing, no escape characters. The KVM server is responsible for mapping terminal input (ASCII, VT100 escape sequences) to HID codes before transmission. The Pico firmware has no knowledge of ASCII — it receives HID pairs and sends USB reports.

Modifier bitmask values:

| Bit  | Key         |
|------|-------------|
| 0x01 | Left Ctrl   |
| 0x02 | Left Shift  |
| 0x04 | Left Alt    |
| 0x08 | Left GUI    |
| 0x10 | Right Ctrl  |
| 0x20 | Right Shift |
| 0x40 | Right Alt   |
| 0x80 | Right GUI   |

## Hardware

### Current development setup

- **KVM server**: Linux laptop (Ubuntu)
- **Keyboard MCU**: Raspberry Pi Pico (RP2040, original, not Pico 2/RP2350)
- **HDMI capture**: MACROSILICON-based USB dongle ($10, USB 2.0, max 1080p MJPEG)
- **Serial link**: CP2102 USB-to-TTL adapter for development; Waveshare multi-channel adapter for production

### Target production hardware

- **KVM server**: Raspberry Pi 3B with powered USB hub
- **Serial link (option A)**: Waveshare USB TO 8CH TTL (SKU 27076, $33) — if UART over Cat5e proves reliable alongside HDMI cables
- **Serial link (option B)**: Waveshare USB TO 8CH RS485 (SKU 28214, $18) with HiLetgo 3.3V RS485 transceivers on each Pico — better noise immunity over longer Cat5e runs
- **HDMI capture**: one MACROSILICON dongle per target machine, connected through powered USB hub
- **Picos**: one per target machine, plugged directly into target's USB port, with Cat5e running back to the Waveshare device server for serial

### Scaling

The design supports up to 8 target machines from a single Pi 3B. Each target requires one Pico, one HDMI capture dongle, and one serial channel. The Waveshare 8-channel serial adapters provide headroom. The Pi 3B's shared USB 2.0 bus (1.2A total, shared with Ethernet) limits simultaneous HDMI streams, but only one stream is captured at a time.

## Pico firmware

The firmware is written in C using the Raspberry Pi Pico SDK (v2.2.0) and TinyUSB.

### How it works

The RP2040's two cores are used independently:

- **Core 0** runs the USB stack. It calls `tud_task()` in a loop, reads keystroke commands from the inter-core FIFO, and sends HID keyboard reports to the host via `tud_hid_keyboard_report()`. Each command produces a key-press report followed by a key-release report.
- **Core 1** manages the UART. An interrupt handler (`UART0_IRQ`) drains received bytes into a lock-free ring buffer. The main loop on core 1 reads bytes from the ring buffer, assembles them into 2-byte `{modifier, keycode}` pairs, and pushes each pair into the inter-core FIFO as a packed `uint32_t`.

### Pin assignments

| Pin   | Function           |
|-------|--------------------|
| GP0   | UART0 TX (command channel) |
| GP1   | UART0 RX (command channel, internal pull-up enabled) |
| GP4   | UART1 TX (debug console via stdio) |
| GP5   | UART1 RX (debug console) |
| USB   | HID Keyboard to target machine |

### USB descriptors

The Pico enumerates as a USB HID keyboard with:

- VID `0xCafe` (TinyUSB test VID — not for distribution)
- Single HID interface, boot-compatible keyboard protocol
- 10ms polling interval
- Remote wakeup capable
- 50mA max power draw

### Building

Prerequisites:

- Raspberry Pi Pico SDK v2.2.0 (installed by the VS Code Pico extension, or manually)
- ARM GCC toolchain (`arm-none-eabi-gcc`, v14.2.Rel1 recommended)
- CMake 3.13+
- Ninja (or Unix Makefiles)

```sh
# Clone and enter the project
cd kvm

# Configure
mkdir build && cd build
cmake ..

# Build
cmake --build . -j8

# Flash: hold BOOTSEL on the Pico, plug into USB, then either:
cp kvm.uf2 /media/$USER/RPI-RP2/     # Linux
# or
cp kvm.uf2 /Volumes/RPI-RP2/         # macOS
# or
picotool load kvm.uf2 -f
```

The build produces `kvm.uf2` in the build directory.

### CMake configuration notes

- `pico_enable_stdio_uart` is enabled (debug output on UART1)
- `pico_enable_stdio_usb` is disabled (USB is reserved for HID)
- `PICO_DEFAULT_UART` is set to 1, with TX on GP4 and RX on GP5, to avoid conflict with the UART0 command channel
- Linked libraries: `pico_stdlib`, `tinyusb_device`, `tinyusb_board`, `pico_multicore`

## KVM server (Go)

A Go program reads terminal input, maps ASCII characters and control codes to HID usage IDs, and writes 2-byte pairs to the serial device connected to the Pico.

### Building

```sh
cd server
go mod tidy
go build -o kvm-send ./cmd/send
```

### Usage (development)

```sh
# Interactive keyboard passthrough
./kvm-send /dev/ttyUSB0     # Linux
./kvm-send /dev/cu.usbserial-XXXXX  # macOS
```

Type into the terminal; keystrokes are sent to the Pico and appear on the target machine. Ctrl+] to quit.

### Dependencies

- `go.bug.st/serial` — serial port access with baud rate configuration
- `golang.org/x/term` — raw terminal mode

## Status

### Working

- Pico enumerates as USB HID keyboard on target machines (Linux, macOS)
- ASCII keyboard input from terminal through serial to Pico to target
- Control characters (Ctrl+A through Ctrl+Z, Escape, Tab, Enter, Backspace)
- HDMI capture dongle validated (MJPEG, 720p, 30fps)
- Dual-core architecture (UART on core 1, USB on core 0, inter-core FIFO)

### Not yet implemented

- Web UI (HTML5 with keyboard capture and MJPEG display)
- HDMI capture integration on the KVM server
- Arrow keys, F-keys, and other multi-byte escape sequences
- Multi-target switching
- RS485 or TTL over Cat5e (currently using direct CP2102 for development)
- Custom EDID to constrain target HDMI output resolution
