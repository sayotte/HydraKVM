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

// Package dispatch routes inbound application messages (Envelopes) to
// registered handlers. It is wire-protocol agnostic: callers (typically the
// HTTP/WebSocket layer) decode whatever wire frame they speak into an
// [Envelope] and call [Router.Dispatch]. JSON unmarshalling of payloads into
// concrete parameter types is handled by [Register] / [RegisterNotification]
// using [encoding/json].
package dispatch
