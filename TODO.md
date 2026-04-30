# TODO

Deferred items that aren't urgent enough to block the current step but
shouldn't be lost.

## Driver lifecycle and error surfacing

Application ref-counts Channel goroutines on attached Clients (start on
first attach, stop on last detach). For Step 1 the Channel goroutine just
drains key events; in Steps 4 and 5, real drivers need to plug into this
lifecycle:

- `KeyEventSink` and `VideoSource` need explicit Open/Close (or
  Start/Stop) hooks so the per-Channel goroutine can release hardware FDs
  on the way down. Today the interfaces have neither — picolink can lazily
  open/close in `ReportKeyEvent` but v4l cannot, since the capture pipeline
  burns CPU on interrupts even with no reader.
- `KeyEventSink.ReportKeyEvent` currently has no error return, and
  `VideoSource.Subscribe` has no error path either. Application can't
  decide whether to reset, reattach, or bounce the goroutine if it never
  hears about the failure. Add error reporting before the picolink/v4l
  drivers ship — likely a per-Channel error channel or a method that
  Application's drainer reads.
- Both items are interface changes, so they need explicit human approval
  before code (per CLAUDE.md "Modifying interfaces").

## Synthetic source per-channel fallback label

Step 4's MJPEG plumbing renders a configurable label on each frame; the
`label` argument is plumbed from `cmd/hydrakvm` (default channel uses
"No Channel Selected", per-channel synthetic sources use the channel's
name). The spec also calls for a fallback variant — when a *real* channel's
video feed goes down, the failover-to-synthetic feed should read
`Channel '<name>' video feed is down`. That requires Step 5's failure
detection plus Application-side rewiring; the label plumbing itself is
already in place, only the failure→fallback wiring is left.

## Browser keyboard.lock and fullscreen capture

`navigator.keyboard.lock([...])` (Chromium-only, requires fullscreen) is
the only way to capture OS-reserved keys like Tab and Esc inside the
browser. The user has explicitly asked that fullscreen NOT be the default
UX; capture this as a deferred enhancement so an opt-in toggle (e.g. a
"fullscreen capture" button) can be added without being forced on
operators.

## README staleness pass

`README.md` build/run sections were refreshed at the end of step 4, but
several other sections still describe pre-step-4 reality and need a
sweep before the next external read:

- **Wire protocol section** says "every message ... is exactly 2 bytes"
  — actually 3 bytes now (`[0xFF SYNC, mod, kc]`, per the firmware
  resync logic added during Wave 2C).
- **Pico firmware "How it works"** says "Each command produces a
  key-press report followed by a key-release report" — no longer true;
  the auto-release was removed so each command is a state snapshot
  (USB HID boot-keyboard semantics).
- **Status / Working** is missing: the web UI, multi-channel switching,
  the `(none)` parking channel, and end-to-end keyboard dispatch from
  browser to picolink.
- **Status / Not yet implemented** still lists "Web UI" and "Arrow keys,
  F-keys, and other multi-byte escape sequences" — both shipped in
  steps 3 and 4.

Architecture diagram and modifier-bitmask table are still correct.

## Multi-key rollover and PiKVM-compatible wire protocol

Two related items, deliberately grouped because their order depends on
what the PiKVM wire protocol turns out to support; not yet investigated.

- **Pressed-usage set in `KbdState`.** Today the Channel drainer tracks
  only the modifier mask, and the Pico wire protocol carries a single
  keycode byte per command. So holding A and then pressing B produces
  successive reports `(0,[A,...])` then `(0,[B,...])`, which the host
  diffs as "A released, B pressed" — wrong for chord-style usage and for
  games (WASD, Shift+WASD). To send a correct 8-byte HID report
  `(mod, [kc1..kc6])` the drainer needs a pressed-usage set so it can
  rebuild the full snapshot on every edge. The spec
  (`docs/code-map.md`, `docs/implementation-plan.md`) already calls for
  this; deferred from Step 4 because nothing exercised it.
- **PiKVM-compatible serial wire protocol.** The current `[0xFF, mod,
  kc]` framing (kvm-side `picolink` and Pico firmware) is bespoke. We
  want to replace it with whatever PiKVM uses end-to-end so HydraKVM
  hardware can interoperate with PiKVM tooling, and vice versa. This
  likely also resolves the rollover protocol question (PiKVM presumably
  carries a full keys array), so the two items may collapse into one
  change.

## Channel key-event write path

The per-Channel serialization goroutine calls `KeyEventSink.ReportKeyEvent`
synchronously. A wedged USB serial device (e.g. picolink unplugged or
hung) can block that call for an unbounded amount of time, which propagates
backpressure all the way to the originating WebSocket reader. Backpressure
is the right default, but unbounded is not — eventually we want a timeout
(or a "drop oldest" / "drop client" policy) so a single bad target can't
freeze the goroutine indefinitely. Decide policy before this becomes a
field problem.
