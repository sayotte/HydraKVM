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
  type KeyEventParams,
  MSG_KEY_EVENT,
} from './protocol.js';

type Sender = (env: Envelope<KeyEventParams>) => void;

export function attachKeyboard(
  target: Window | HTMLElement,
  send: Sender,
): () => void {
  const handler = (raw: Event): void => {
    if (!(raw instanceof KeyboardEvent)) return;
    raw.preventDefault();
    const code = raw.code;
    if (code === '') {
      console.debug('[keyboard] ignored event with empty code', raw);
      return;
    }
    const type: KeyEventParams['type'] =
      raw.type === 'keydown' ? 'keydown' : 'keyup';
    console.debug(
      `[keyboard] capture type=${type} code=${code} repeat=${raw.repeat}`,
    );
    send({
      type: MSG_KEY_EVENT,
      payload: { type, code },
    });
  };
  target.addEventListener('keydown', handler);
  target.addEventListener('keyup', handler);
  return () => {
    target.removeEventListener('keydown', handler);
    target.removeEventListener('keyup', handler);
  };
}
