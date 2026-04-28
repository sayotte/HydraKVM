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

## Channel key-event write path

The per-Channel serialization goroutine calls `KeyEventSink.ReportKeyEvent`
synchronously. A wedged USB serial device (e.g. picolink unplugged or
hung) can block that call for an unbounded amount of time, which propagates
backpressure all the way to the originating WebSocket reader. Backpressure
is the right default, but unbounded is not — eventually we want a timeout
(or a "drop oldest" / "drop client" policy) so a single bad target can't
freeze the goroutine indefinitely. Decide policy before this becomes a
field problem.
