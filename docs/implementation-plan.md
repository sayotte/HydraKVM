# HydraKVM — implementation plan

Steps are ordered so every gate can be satisfied before the next step starts. Each step names the scope of what gets built and the verification that must pass before moving on.

## Package dependency reference

- `cmd/hydrakvm`
  - `config`
  - `auth`
  - `internal/dispatch`
  - `internal/http`
    - `internal/http/websocket`
    - `internal/http/web`
  - `internal/kvm`
  - `internal/picolink`
  - `internal/v4l`
  - `internal/synthetic`
- `config` -> (none)
- `auth` -> (none)
- `internal/dispatch`
  - `internal/kvm`
- `internal/http`
  - `internal/http/websocket`
  - `internal/dispatch`
  - `config`
  - `auth`
- `internal/http/websocket`
  - `internal/kvm`
- `internal/http/web`
  - `internal/http/websocket`
- `internal/kvm` -> (none)
- `internal/picolink`
  - `internal/kvm`
- `internal/v4l`
  - `internal/kvm`
- `internal/synthetic`
  - `internal/kvm`

## Order of implementation

### Step 1: Core scaffolding
- `internal/kvm`
  - Its key structs:
    ```go
    type StreamShape struct {
        Codec     string // "mjpeg", "h264"
        MIMEType  string // "image/jpeg", "video/mp4; codecs=..."
        Framing   string // "multipart" (per-frame Content-Type+Length, MJPEG style) | "continuous" (single response, frames concatenated, fMP4 style)
        Width     int
        Height    int
        Framerate int
        Profile   string // h264 only, empty for mjpeg
    }

    type VideoFrame struct {
        Data      []byte
        IsKey     bool      // can a new subscriber start here?
        PTS       time.Duration
    }

    type KeyEvent struct {
      // Code is a KeyboardEvent.code-style identifier (e.g. "KeyA", "ShiftLeft", "F11").
      // Translation from Code to USB HID usage integers happens inside the KeyEventSink
      // implementation (e.g. picolink). Application decides WHICH events to send and WHEN
      // (state-machine: tracks held modifiers, decides whether a given browser event
      // should produce an outbound KeyEvent at all, etc.). The KeyEventSink is told what
      // to do; it doesn't decide.
      Code string
      Kind string // "down" | "up" | (later) "tap"
      // ...
    }

    type Client struct {
      VideoOut FrameSink
      Outbound MessageWriter   // used by Application to push notifications back to this client
      // ...
    }

    type Channel struct {
      VideoIn  VideoSource
      KeyOut   KeyEventSink
      KbdState KeyboardState  // modifier bitmap, pressed-usage set, policy flags
      // Channel owns a single goroutine that drains an unbuffered chan KeyEvent
      // and calls KeyOut.ReportKeyEvent in order. Multiple Clients writing
      // concurrently get serialized by the channel send; backpressure flows
      // upstream to the WebSocket goroutine, which is the desired behavior.
      // The goroutine is launched by Application on first Client attach and
      // torn down on last detach (ref-counted lifecycle); Channel itself does
      // not own when it runs.
      // ...
    }

    type KeyboardState struct {
      // modifier bitmap, pressed-usage set, etc.
      // ...
    }

    type Application struct {
      // owns the Client<->Channel association:
      //   map[*Channel]map[*Client]struct{}  (or equivalent)
      // mutated atomically under a single Application-level lock during
      // SwitchChannel. Clients themselves do not carry a current-channel
      // pointer — Application is the single source of truth.
      //
      // Also owns Channel goroutine lifecycle: launches Channel.Run on
      // first Client attach, cancels its context on last detach. This
      // releases hardware FDs (HDMI capture, serial port) when nothing is
      // watching and gives Application the seam to handle driver errors.
      // ...
    }
    func NewApplication(baseCtx context.Context) *Application
    func (a *Application) SwitchChannel(ctx context.Context, p SwitchChannelParams) (SwitchChannelResult, error)
    func (a *Application) RecordKeyEvent(ctx context.Context, p KeyEventParams) error
    ```
  - The Message types used throughout the system
    ```go
    const (
        MsgSwitchChannel  = "switch_channel"
        MsgKeyEvent       = "keyevent"
        MsgClientUpdate   = "client_update"
    )

    type SwitchChannelParams struct { /* ... */ }
    type SwitchChannelResult struct { /* ... */ }
    type KeyEventParams      struct { /* ... */ }
    ```
  - Its interfaces
    ```go
    type VideoSource interface {
      Shape() StreamShape
      InitData() []byte                 // SPS+PPS for h264, nil for mjpeg
      RequestKeyframe() error           // no-op for mjpeg; plausibly an ioctl for h264
      Subscribe(ctx context.Context) <-chan VideoFrame
    }

    type FrameSink interface {
      WriteFrame(vf VideoFrame)
    }

    type KeyEventSink interface {
      // Receives abstract KeyEvents (Code strings + Kind) from Application and
      // is responsible for translating them into whatever wire protocol drives
      // the actual keyboard hardware (USB HID codes for picolink). Stateless
      // with respect to "which keys are held" — that lives in Application.
      ReportKeyEvent(ke KeyEvent)
    }

    type MessageWriter interface {
      // Used by Application to push notifications to a Client (e.g. "channel
      // X went down, you've been switched to fallback"). For Webclients this is
      // bridged to outbound WebSocket frames by http/websocket.Codec.
      WriteMessage(msgType string, payload any) error
    }
    ```
- `internal/dispatch`
  - Its key structs:
  ```go
  type Envelope struct {
    Type    string          `json:"type"`
    ID      string          `json:"id,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
  }

  type Handler func(ctx context.Context, payload json.RawMessage) (any, error)

  type Router struct { /* ... */ }
  func NewRouter() *Router
  func (r *Router) Handle(msgType string, h Handler)
  func (r *Router) Dispatch(ctx context.Context, env Envelope) (*Envelope, error)

  func Register[P, R any](r *Router, msgType string, fn func(context.Context, P) (R, error))
  func RegisterNotification[P any](r *Router, msgType string, fn func(context.Context, P) error)
  ```
- Unit tests for both packages
- Integration tests to show that:
  - calling Dispatch with a given Envelope invokes the expected Application method with the right params
  - two Clients driving the same Channel concurrently produce serialized, non-torn frames into the Channel's KeyEventSink (one writer per Channel, or equivalent), and per-Client state (held modifiers, etc.) does not bleed between clients

### Step 2: main executable and wiring stubs
- `config`
  - Its key structs:
  ```go
  type Config struct {
    HTTPConfig HttpServerConfig
    AuthConfig AuthConfig
    Channels []ChannelConfig
    
  }
  func (c *Config) FromYAML(path string) error
  func (c *Config) ToYAML(path string) error
  func (c *Config) Validate() []error

  type ChannelConfig struct {
    Video VideoSourceConfig
    Keys KeyEventSinkConfig
  }

  type VideoSourceConfig struct {
    Type string // e.g. "synthetic", "MAXIMOS"
    DevicePath string // e.g. /dev/video5
  }

  type KeyEventSinkConfig struct {
    Type string // e.g. "picosink"
    DevicePath string // e.g. /dev/ttyusbserial.234lskdf
  }

  type HttpServerConfig struct {
    ListenAddr // net.Dial format
  }

  type AuthConfig struct {
    AuthProvider string
    AuthProviderConfig json.RawMessage
  }
  ```
- `internal/http`
  - Its key struct:
  ```go
  type HttpServer struct {
    netServer  *net/http.Server
    Config     config.HttpServerConfig
    Dispatcher *dispatch.Router
  }
  func (h *HttpServer) ListenAndServe() error
  ```
  - Stub implementation of `ListenAndServe`, just sleeps forever
- `internal/synthetic`
  - Stub implementation of `kvm.VideoSource`-- emits empty frames for now
- `internal/picolink`
  - Stub implementation of KeyEventSink-- ignores incoming events
- `internal/http`
  - Stub implementation 
- `cmd/hydrakvm`
  - CLI argument for config file path (with a default)
  - CLI help text
  - Code to:
    - load config from a file and validate it
    - create a `Channel` with a `synthetic` video source and `picolink` KeySink
    - create an `auth.Provider`
    - create an `HttpServer` with the `auth.Provider`
    - create a `kvm.Application`
    - create a `dispatch.Router` and register various Application methods with it (they're all stubs)
    - launch the `kvm.Application`'s main loop
### Step 3: Working web interface
- Create internal scaffolding for web client-- should load from a static directory and have the outline of the eventual Webclient, with non-firing dropdowns / buttons / keyboard input
- Implement serving up the Webclient from `HttpServer`
- Implement WebSocket connection from Webclient->Server (opens connection, then does nothing)
- Define `http/websocket.Codec`
- Implement channel-switch request from Webclient->Server
- Implement keystroke-event sending from Webclient->Server
- Implement Webclient initiation flow:
  - Browser GETs the page; server returns the Webclient HTML/JS/CSS
  - Webclient calls back to the server (HTTP); server returns a WebSocket URL
  - Webclient connects to that WebSocket URL; server dispatches NewClient
  - Server sends a message over the WebSocket containing the URL for the MJPEG video stream
  - Webclient connects its `<img>` tag to that MJPEG URL
- Implement `internal/synthetic` video stream
- Implement NewClient in `kvm.Application`: connect Webclient to Channel with a `synthetic` video stream upon new client connection, and swap to new (perhaps random identity?) Channel upon channel switch
- Implement keystroke-event sending, dispatching to `Application` (no decode at this point)
- Webclient styling and structure: Tailwind CSS with recycled theme; semantic layout for the viewport, channel selector, control-sequence button row, and a connection-status indicator. No fancy animations; just an intentional-looking page.
- Keyboard capture hardening:
  - `keydown`/`keyup` listeners attached to `window` (or a focused capture element with `tabindex="0"`)
  - `preventDefault()` on every captured event so the browser does not act on it locally
  - Use `e.code` (physical key location) for Code values, not `e.key`
  - Filter or expose `e.repeat === true` per a knob the spec for this step decides; default to forwarding so the target sees auto-repeat
  - Call `navigator.keyboard.lock([...])` when the page enters fullscreen so chords like Tab/Escape can be captured on Chromium; silently skip on browsers that don't support it
  - Visible focus indicator on the capture element so the user knows when keystrokes are being forwarded
- Control-sequence buttons fire real events: each button (Ctrl-Alt-Del, Alt-Tab, Win, Esc, …) emits a sequence of `KeyEvent` messages — press events for each key, then release events in reverse order — using the same WS path as physical keystrokes. No special server opcode for canned sequences.
- Channel selector: populated from a server-injected list (rendered into the page by `html/template`); switching dispatches `MsgSwitchChannel` over the WS.
- Connection-status UI: shows current state (`connecting`, `connected`, `reconnecting`, `disconnected`), driven by WS lifecycle events and inbound `client_update` messages.
- WebSocket reconnect with exponential backoff (capped); on reconnect, server treats it as a new Client (per the existing NewClient flow) — no resumption attempt in v1.
- Add access-logs and logging of key HttpServer transitions and occurrences

### Step 4: Working KeyEvent -> actual keyboard input flow
- Implement `picolink` writing: test harness can send a known sequence of USB HID reports that includes modifier keys with their down/up staggered around character inputs; requires manual verification
- Implement decoding of keystroke events from websocket into `kvm` messages
- Implement encoding of `kvm` messages into USB HID reports within the `picolink` package
- Implement keystroke message dispatch and end-to-end keystroke sending: from Webclient to keyboard host
- Add channel attach/detach logging, `picolink` failure logging, and logging of key Application transitions and occurrences

### Step 5: Working Video
- Implement `v4l` USB capture reading: test harness can grab frames and write to a file which can be played back manually
- Implement `v4l` -> Webclient streaming; verify in-browser display behaves reasonably
- Add `v4l` failure logging
- Implement default-stream fallback: `main` injects a default `VideoSource` (in MVP, a `synthetic` "channel down" feed) into `kvm.Application` at startup. When a channel's real `VideoSource` reports failure (cable unplug, device error, read timeout), `Application` rewires the affected Channel's connected Clients to the injected default and pushes a notification through each Client's `MessageWriter` (`MsgClientUpdate` with a payload describing which channel went down). On recovery, `Application` rewires Clients back to the real source and pushes another notification. `kvm.Application` does not import `internal/synthetic` — the default is supplied as a `VideoSource` interface value.
- Verification: manually unplug the HDMI dongle mid-stream; UI shows the fallback feed within one frame interval and a `client_update` message appears on the WebSocket. Replug; recovery without browser reconnect.

### Step 6: Channel switching
- Implement multi-channel: `kvm.Application` only activates a channel for a Client after they send an initial channel-switch message selecting it
- Implement basic channel switch: can switch between `synthetic` video channels
- Implement USB channel switching: can switch between a USB channel and a `synthetic` channel; USB channels not being read from must be properly disconnected via v4l
- Log channel switches

### Step 7: Auth + Login
- Implement local user database with ability to read and write per-user secrets stored using `golang.org/x/crypto/bcrypt`
- Implement CLI parameter for creating/updating a user with password read from stdin
- Create Webclient login page and HttpServer endpoint where Webclient must POST credentials
- Implement HttpServer credential validation, wire to login POST
- Implement HttpServer in-memory session database (thread safe), session creation and deletion methods
- Connect successful credential validation to POST response with session cookie / header / whatever
- Create middleware for session-validation on HTTP handlers; redirect to login page if invalid or nonexistent; wire into all handlers except the Webclient serving page and the login endpoint
- Log all auth attempts, including username (no password in logs) and result/status

### Step 8: Reverse-proxy and operational setup
- Create Caddyfile for front-ending the `HttpServer`, with mTLS on incoming connections (these will be using a self-managed CA, connections incoming from an internal load-balancer)
- Create Dockerfile for containerizing `hydrakvm`
- Create compose.yml for running `hydrakvm` and Caddy; logfiles must be written to a volume; needed USB devices must be accessible to `hydrakvm`
