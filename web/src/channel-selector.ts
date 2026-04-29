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

import {
  type Envelope,
  type SwitchChannelParams,
  MSG_SWITCH_CHANNEL,
} from './protocol.js';

type Sender = (env: Envelope<SwitchChannelParams>) => void;

export function attachChannelSelector(
  selectEl: HTMLSelectElement,
  send: Sender,
): () => void {
  const handler = (): void => {
    const id = selectEl.value;
    if (id === '') return;
    send({
      type: MSG_SWITCH_CHANNEL,
      payload: { channel_id: id },
    });
  };
  selectEl.addEventListener('change', handler);
  return () => {
    selectEl.removeEventListener('change', handler);
  };
}
