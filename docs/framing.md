# HydraKVM server — framing doc

## Context

HydraKVM is three top-level components:

- Hardware / firmware
  - **HDMI capture** - a commodify HDMI-to-USB dongle compatible with V4L
  - **Pico** (`keyboard/`) — a USB HID keyboard that receives framed
      `{modifier, keycode}` pairs over UART0 and replays them to a target machine.
      Working and validated.
- The server
  - Written in Go, communicates with both the hardware and with the client
- The client
  - Web client written in plain HTML + TypeScript

Production is meant to be a Raspberry Pi 3B driving up to 8 headless target
machines, each with its own Pico + HDMI capture dongle, accessed from a browser
over HTTPS. Getting from "CLI keystroke forwarder" to "multi-channel
browser-accessible KVM" is the work this plan frames.

## What goes into the server

Three coordinated subsystems inside a single Go process:

1. **Channel layer** — owns all per-target resources. For each of N channels:
   an HID output sink (UART to that channel's Pico) and a video frame source
   (V4L2 from that channel's capture dongle). Exposes a uniform interface to
   upstream code regardless of whether the hardware is real, mocked, or
   currently offline.

2. **Session layer** — authenticates operators, tracks which channel each
   active operator session is currently bound to, and mediates between
   WebSocket/MJPEG connections and the channel layer. This is where the rule
   "you type into the channel you're watching" is enforced, atomically.

3. **Web layer** — HTTP/HTTPS server with three kinds of endpoints: static
   page rendering (login + main UI), long-lived MJPEG streaming, and
   WebSocket for bidirectional control + status. Entirely stateless with
   respect to channels — always consults the session layer.

## Whiteboard view

```
 ┌─ Browser ─────────────────────────────────────────────────────┐
 │  <img src="/stream">   ◀── MJPEG ─────┐                       │
 │  WebSocket /ws         ◀─ keystroke/ack/status/switch ─┐      │
 └────────────────────────────────────────┬───────────────┼──────┘
                                          │               │
                                          ▼               ▼
 ┌─ Go server ─────────────────────────────────────────────────────┐
 │                                                                 │
 │  Web layer:  http.Server  +  websocket upgrader                 │
 │      │                              │                           │
 │      │ "what channel is this        │ "switch me to channel 3"  │
 │      │  session bound to?"          │ "keystroke: KeyA down"    │
 │      ▼                              ▼                           │
 │  Session layer:  sessions[token] → { channel, since, ... }      │
 │      │                              │                           │
 │      │ frame?                       │ send HID pair             │
 │      ▼                              ▼                           │
 │  Channel layer:  channels[N] = { FrameSource, HIDSink }         │
 │          │                                  │                   │
 │          ▼                                  ▼                   │
 │     V4L2 dongle N                      UART to Pico N           │
 │     (or synthetic)                     (or null sink)           │
 └─────────────────────────────────────────────────────────────────┘
                  │                                   │
                  ▼                                   ▼
           HDMI capture dongles            Picos (1 per target)
                  │                                   │
                  ▼                                   ▼
                         ┌── Target machine N ──┐
                         │  HDMI out   USB in   │
                         └──────────────────────┘
```

## Key architectural decisions

### D1 — Single persistent MJPEG + single WebSocket per browser session

Video goes over a long-lived `<img src="/stream">`; control goes over a
WebSocket. The `<img>` src is set once and never changed. Channel switching
is a WS message that repoints *both* video and keyboard on the server,
atomically, under one session identity. Keeps the browser stupid and makes
"video shows channel A while keystrokes go to channel B" structurally
impossible. (Full reasoning discussed earlier in the conversation.)

### D2 — Channel is the unit of resource ownership

The hardware side is a first-class subsystem with its own module, not an
implementation detail of HTTP handlers.

A channel is a single object with a lifecycle, owning one `HIDSink` and one `FrameSource`,
plus health/status for both. Sessions bind to channels by number. Channels
are created at startup from config and persist for the life of the process;
they do not come and go with operator activity.

### D3 — Both hardware legs have a synthetic implementation

`FrameSource` has a real V4L2 implementation and a synthetic generator
implementation. `HIDSink` has a real UART implementation and a log-only
null implementation. This isn't only for dev — it's also the runtime
behavior when hardware is absent or unhealthy. A channel whose capture
dongle is unplugged falls back to synthetic frames that say so; a channel
whose Pico is unresponsive accepts keystrokes but logs + drops them and
reports `hid: down` on the status channel. No endpoint ever returns a
hard HTTP error for missing hardware — degraded state is always
representable in the normal data flow. This matters because the whole
system will routinely run with 0–8 legs of hardware attached.

### D4 — Keyboard input translation is two distinct layers

The browser gathers `KeyboardEvent.code` values (e.g. `"KeyA"`, `"ShiftLeft"`, `"F11"`) plus
explicit `down`/`up` events from the user, and must translate those into
the wire-protocol sent over the WebSocket to the HTTP server (which also includes control
messages such as channel-switching).

Once received by the HTTP server, those event values must be translated into USB HID codes
(modifier + key) to be sent over the UART to the keyboard emulator.

### D5 - Operator model: multiple viewers' inputs are serialized

The possibility of multiple users utilizing the same KVM channel at
the same time is very plausible, and simultaneous access might be
useful. Two processes trying to write over the same UART at once could
produce garbage though, so all inputs flowing to that UART must go through
a single writer in order to serialize them-- the writer will only ever
write valid frames, so at worst the users will have their inputs interleaved
to annoying effect. If that proves problematic, we can add a control
status / grabbing feature later on.

### D6 — Session state is in-memory and server-local

No database for sessions. A `map[token]*Session` behind a
mutex is sufficient for ≤8 channels and a handful of operators, this application
is intrinsically not horizontally scalable (only one writer to each UART), and needing to redo
login on process restart is acceptable. Auth credentials live in a config file
initially, with secrets encrypted using bcrypt.

### D7 — Graceful degradation over strict health-gating

The server starts and serves traffic even if zero channels have healthy
hardware. Channels report their own health over the WS status channel;
the UI shows what's up and what's down. This keeps dev cycles short
(no need to stub hardware to get the page to load) and matches the
operational reality that hardware will be partially offline at various
times.

### D8 - Pure keypress events over the wire

Every keydown/keyup event the user inputs into their browser is sent
along to the host machine faithfully. USB HID semantics require maintaining
modifier bits in subsequent other-key keypresses. This may be implementable
in the Go server; otherwise it will be handled in the Pico firmware.

This decision was made because a KVM is more useful if it can interact with 
the machine's BIOS, bootloader and so forth, many of which require depressing
a key and keeping it depressed.

## Non-goals (explicit)

- Multiple server instances. One process, one Pi.
- Horizontal scaling, stateless sessions, session replication.
- Mouse support. Keyboard + video only. (Pico firmware doesn't do
  mouse HID today; the wire protocol and USB descriptors would both
  need to change.)
- Audio. HDMI capture dongles pass audio but we don't route it.
- Recording / scrollback of video or keystrokes.
- Per-operator fine-grained permissions. All logged-in operators can
  access all channels.
- In-browser authentication flows beyond password login (no OAuth,
  no 2FA in v1).
- USB over Cat5e (far outside current scope; the Pico + cable runs
  back to the server are the boundary).

## Constraints and realities worth naming

- **Browser capture constraints (D4-related).** Some keys cannot be
  intercepted by any amount of JS: OS-level chords like Cmd-Q,
  Alt-Tab, Ctrl-Alt-Del on Windows, F11. `navigator.keyboard.lock()`
  helps in Chromium + fullscreen only. For the unreachable cases we
  expose **canned-sequence buttons** in the UI (Ctrl-Alt-Del,
  Alt-Tab, Win, Esc, …). The button path and the keyboard path both
  emit the same `{op:"key",code,type}` messages to the WS, so the
  server doesn't care which triggered them.
- **UART bandwidth.** At 115200 8N1 that's ~11.5 kB/s. Our framed
  pairs are 3 bytes each (`0xFF` + mod + kc). Even at sustained
  typing that's nowhere near saturating the link. No flow-control
  design needed.
- **MJPEG is the wrong long-term format** (one TCP connection per
  viewer, no key frames because every frame is a key frame, no
  easy seeking). For v1 it's right anyway: the dongle emits it
  natively, the browser decodes it in `<img>`, and we don't need
  the features it lacks.
- **V4L2 access on the Pi.** Capture devices enumerate as
  `/dev/videoN`. Mapping "channel 3" to `/dev/video5` is config,
  not discovery; dongles are identified by USB path (bus/port),
  not by the possibly-renumbering `videoN` index.
- **One operator, many tabs.** A single human opening two browser
  tabs creates two sessions.

## What a successful v1 looks like

- Operator opens the URL, logs in.
- Main page loads. Channel dropdown shows all 8 channels with
  health badges (e.g. channel 3: "capture down", channel 5:
  "ok").
- Operator picks channel 5. Live video appears. Keyboard focus
  on the page forwards keystrokes to target machine 5.
- Operator picks channel 3. The `<img>` keeps running;
  synthetic "capture down" frames appear instantly. Operator's
  keystrokes still go somewhere sensible (either dropped with
  status indication, or rejected with a visible UI state —
  decide during design).
- Operator clicks "Ctrl-Alt-Del" button; target machine 5
  receives it (after switching back).
- Unplug channel 5's HDMI dongle at runtime. Within one frame
  interval the stream shifts to synthetic "capture down" frames
  for that channel. Replug: recovery without reconnect.

## What this doc is not

This is framing only. It does not specify:

- File layout, package names, struct shapes, or function
  signatures. (Module map — separate doc.)
- The order of building. (Implementation plan — separate doc.)
- Concrete types or protocols for the first step.
  (Technical spec for step 1 — separate doc.)

Those come next, in that order.
