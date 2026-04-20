# HydraKVM — code map

## Module map

| Package | Responsibility | Key types | Depends on |
|---|---|---|---|
| `cmd/hydrakvm` | Main binary entrypoint. Responsible for wiring other packages together, process initiation and restart/recovery. | - | everything |
| `config` | Application configuration definitions and validation routines. | `Config` | - |
| `auth` | Verifies credentials and authorization policy queries. Also responsible for the concept of an "account". | `Authenticator`, `Authorizer`, `Account` | - |
| `internal/dispatch` | Routes inbound messages (from client) to internal application behavior. | `Router` | `kvm` |
| `internal/http` | Webserver implementation. | `Server`, `Session` | `config`, `auth`, `kvm`, `websocket`, `dispatch` |
| `internal/http/websocket` | Framing and content definitions for Websocket communications | `Codec` | `kvm` |
| `internal/http/web` | Embedded TypeScript for web client | - | - |
| `internal/kvm` | Defines application behavior. | `Application`, `Channel`, `Client`, `FrameSource`, `FrameSink`, `KeySource`, `KeySink` | - |
| `internal/picolink` | Implements `kvm.KeySink` for the serial protocol sent to the Pico keyboard emulator. Also implements USB HID conversion unless/until another KeySink also uses USB keycodes. | `Keyboard` | - |
| `internal/v4l` | Implements `kvm.FrameSource` for v4l video capture devices. | `MJPEGStream` | - |
| `internal/synthetic` | Synthetic `kvm.FrameSource` implementations: a "channel down" fallback video feed, and a dev test-pattern source used when no capture hardware is attached. Expected to be replaced or joined by driver-specific packages as additional video codecs are supported. | - | `kvm` |

## Wiring / dependency injection
All wiring is done by `cmd/hydrakvm`.

The core concerns are captured in `internal/kvm` so it has no outbound dependencies-- it provides interfaces that other packages implement.

### Channels
Per configuration at startup, `main()` creates `Channel` objects which consist of one `KeySink` (where to send keystrokes) and one `FrameSource` (where to get streaming video), plus a `KeyboardState` struct (modifier bitmap, pressed-usage set, and policy flags) that `Application` mutates in response to `KeySource` events and uses to drive the `KeySink`. For MVP the `KeySink` will be a `picolink.Keyboard`, and the `FrameSource` will be a `v4l.MJPEGStream`.

### Clients
Client objects are created by `http` when a web client connects to the two streaming connections for messages (websocket) and video (mjpeg). The latter implements `FrameSink`, which will be connected to various `FrameSource` feeds by `kvm.Application` when channels are selected. Client objects are cleaned up when the TCP connections associated with them go away, e.g. due to a full-page refresh or the user closing the browser tab. Note that this is different from an `http.Session`, which may have multiple Clients (one per open browser tab) but only a single authentication token.

`FrameSink` is an interface (shape TBD, expected to be roughly `WriteFrame([]byte)`) rather than a bare Go channel — implementations are free to use a channel internally, but also free to use another mechanism. Backpressure strategy is deferred to the spec; the expected behavior is that a slow reader causes frames to be dropped, not accumulated.

Each Client also exposes a `MessageWriter` field that `Application` uses to push outbound control messages (e.g. "channel is down", "controller status changed") to the client. For the webclient, this is bridged through `websocket.Codec` onto the WS connection; for the CLI client it can discard or log.

### Dispatch
Client interaction with the application is conducted with messages of varying types. At startup, `main()` creates a `dispatch.Router` and registers `kvm.Application` methods for the various message types associated with them (defined within `kvm`). Upon receipt of a message, its recipient (currently the `http.Server`) calls `dispatch.Router.Dispatch` on the message which invokes the correct domain behavior.

### Webserver
The Webserver is responsible for providing an HTTP API that can do these things:
- serve static web-client content
- process a login request and return a session token
- promote an authenticated request to a Websocket, and stream messages (e.g. keypress events) to and from the web-client over it
- upon creation of a Websocket, create a Client object (using a factory from `application`) to wrap the Websocket
- upon creation of a Client, send the client a message containing a URL to connect to for streaming video, and when that connection comes in associate it with the correct Client
- stream continuous frames to an `<img>` tag in the web-client
- tear down the Client of any web-client whose Websocket connection dies
- dispatch incoming Websocket messages to application code

It depends on `auth` to verify the authenticity of credentials, and `websocket` to encode/decode internal (`application`) messages into the format actually sent across the wire to the web-client.

HTTP Handlers are unaware of what happens with messages it dispatches to the application, or the meaning of messages being sent to the client (which, in fact, are sent directly through the `Client` object). In the case of a streaming response, the HTTP handler copies bytes from a channel on the `Client` into an `http.Flusher` (typecast from the `http.ResponseWriter`).

### Webclient
The webclient is responsible for negotiating a login session with the server, presenting the video feed to the user, capturing their keyboard input and forwarding it as `websocket` protocol-messages, and providing the user working buttons and selectors for changing channels, sending awkward modifier key combinations, and so on.

After login succeeds, the web client initiates a Websocket connection to the server. When that is established, the server sends a message to the webclient with a URL which will provide a video feed. The client connects its `<img>` video feed to that URL, and once that is established it sends a message (perhaps waiting for user input) selecting a channel to connect to.

### KVM / Application
Responsible for connecting Clients to Channels, maintaining appropriate state for that connection (e.g. the status of modifier keys such as CapsLock), and responding to events/commands and changes in the environment (e.g. a cable being unplugged) in a reasonable way (e.g. playing an obviously-connected but synthetic video stream when the real video-capture stream is unavailable).
