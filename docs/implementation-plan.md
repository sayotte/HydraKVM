# HydraKVM — implementation plan

Steps are ordered so every gate can be satisfied before the next step starts. Each step names the scope of what gets bWebclientlt and the verification that must pass before moving on.

## Package dependency reference

- `cmd/hydrakvm`
  - `config`
  - `auth`
  - `internal/dispatch`
  - `internal/http`
    - `internal/http/websocket`
    - `internal/http/web`
  - `internal/kvm`
    - `internal/synthetic`
  - `internal/picolink`
  - `internal/v4l`
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
- `internal/kvm`
  - `internal/synthetic`
- `internal/picolink` -> (none)
- `internal/v4l` -> (none)
- `internal/synthetic` -> (none)

## Order of implementation

### Step 1: Core scaffolding
- `internal/kvm`
  - Its key structs:
    ```go
    type StreamShape struct {
        Codec     string // "mjpeg", "h264"
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
      // ...
    }

    type Client struct {
      VideoOut FrameSink
      // ...
    }

    type Channel struct {
      VideoIn VideoSource
      KeyOut KeyEventSink
      // ...
    }

    type Application struct {
      // ...
    }
    func (a *App) SwitchChannel(ctx context.Context, p SwitchChannelParams) (SwitchChannelResult, error)
    func (a *App) RecordKeystroke(ctx context.Context, p KeystrokeParams) error)  
    ```
  - The Message types used throughout the system
    ```go
    const (
        MsgSwitchChannel = "switch_channel"
        MsgKeyEvent     = "keyevent"
        MsgWebclientUpdate      = "Webclient_update"
    )

    type SwitchChannelParams struct { /* ... */ }
    type SwitchChannelResult struct { /* ... */ }
    type KeyEventParams     struct { /* ... */ }
    ```
  - Its interfaces
    ```go
    type VideoSource interface {
      Shape() StreamShape
      InitData() []byte                 // SPS+PPS for h264, nil for mjpeg
      RequestKeyframe() error           // no-op for mjpeg; plausibly an ioctl for h264
      Subscribe(ctx context.Context) <-chan Frame
    }

    type FrameSink interface {
      WriteFrame(vf VideoFrame)
    }

    type KeyEventSink interface {
      ReportKeyEvent(ke KeyEvent)
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
- Integration tests to show that calling Dispatch with a given Envelope invokes the expected Application method with the right params

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
- Implement client initiation flow: connect to server, get Webclient, Webclient phones home, server sends websocket URL, connects to websocket, Server dispatches NewClient
- Implement `internal/synthetic` video stream
- Implement NewClient in `kvm.Application`: connect Webclient to Channel with a `synthetic` video stream upon new client connection, and swap to new (perhaps random identity?) Channel upon channel switch
- Implement keystroke-event sending, dispatching to `Application` (no decode at this point)
- Add access-logs and logging of key HttpServer transitions and occurrences

### Step 3: Working KeyEvent -> actual keyboard input flow
- Implement `picolink` writing: test harness can send a known sequence of USB HID reports that includes modifier keys with their down/up staggered around character inputs; requires manual verification
- Implement decoding of keystroke events from websocket into `kvm` messages
- Implement encoding of `kvm` messages into USB HID reports within the `picolink` package
- Implement keystroke message dispatch and end-to-end keystroke sending: from Webclient to keyboard host
- Add channel attach/detach logging, `picolink` failure logging, and logging of key Application transitions and occurrences

### Step 4: Working Video
- Implement `v4l` USB capture reading: test harness can grab frames and write to a file which can be played back manually
- Implement `v4l` -> Webclient streaming; verify in-browser display behaves reasonably
- Add `v4l` failure logging

### Step 5: Channel switching
- Implement multi-channel: `kvm.Application` only activates a channel for a Client after they send an initial channel-switch message selecting it
- Implement basic channel switch: can switch between `synthetic` video channels
- Implement USB channel switching: can switch between a USB channel and a `synthetic` channel; USB channels not being read from must be properly disconnected via v4l
- Log channel switches

### Step 6: Auth + Login
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
