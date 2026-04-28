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

package kvm

// Message type constants. These are the canonical strings used in the
// Envelope.Type field by [internal/dispatch] and on the wire by
// internal/http/websocket.
const (
	MsgSwitchChannel = "switch_channel"
	MsgKeyEvent      = "keyevent"
	MsgClientUpdate  = "client_update"
)

// SwitchChannelParams is the inbound payload for [MsgSwitchChannel].
type SwitchChannelParams struct {
	ChannelID string `json:"channel_id"`
}

// SwitchChannelResult is the response payload returned by
// [Application.SwitchChannel].
type SwitchChannelResult struct {
	ChannelID string `json:"channel_id"`
}

// ClientUpdateParams is the outbound payload for [MsgClientUpdate]
// notifications pushed from Application to a Client.
type ClientUpdateParams struct {
	ChannelID string `json:"channel_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
}
