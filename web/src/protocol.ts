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

export const MSG_SWITCH_CHANNEL = 'switch_channel' as const;
export const MSG_KEY_EVENT = 'keyevent' as const;
export const MSG_CLIENT_UPDATE = 'client_update' as const;
export const MSG_MJPEG_URL = 'mjpeg_url' as const;

export interface Envelope<T = unknown> {
  type: string;
  id?: string;
  payload?: T;
}

export interface SwitchChannelParams {
  channel_id: string;
}

export type KeyEventType = 'keydown' | 'keyup';

export interface KeyEventParams {
  type: KeyEventType;
  code: string;
}

export interface MJPEGURLPayload {
  url: string;
}

export interface ConnectResponse {
  ws_url: string;
}
