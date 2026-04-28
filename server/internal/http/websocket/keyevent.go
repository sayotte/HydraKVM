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

package websocket

// KeyEventParams is the wire shape for an inbound MsgKeyEvent payload.
// The Go field names match W3C KeyboardEvent semantics (Type / Code) while
// JSON tags match what the browser KeyboardEvent fields are named on the
// wire.
type KeyEventParams struct {
	Type string `json:"type"` // W3C KeyboardEvent.type, e.g. "keyup" / "keydown"
	Code string `json:"code"` // W3C KeyboardEvent.code, e.g. "KeyA" / "Enter"
}
