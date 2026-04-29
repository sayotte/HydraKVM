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

const SEQUENCES: Record<string, string[]> = {
  'Ctrl-Alt-Del': ['ControlLeft', 'AltLeft', 'Delete'],
  'Alt-Tab': ['AltLeft', 'Tab'],
  'Win': ['MetaLeft'],
  'Esc': ['Escape'],
  'Ctrl-C': ['ControlLeft', 'KeyC'],
};

const BUTTON_CLASSES =
  'px-3 py-1.5 rounded border border-sand bg-linen text-charcoal ' +
  'hover:bg-sand hover:border-slate text-sm font-mono transition-colors';

export function attachControlSequences(
  containerEl: HTMLElement,
  send: Sender,
): () => void {
  const buttons: HTMLButtonElement[] = [];
  const handlers = new Map<HTMLButtonElement, () => void>();

  for (const [label, codes] of Object.entries(SEQUENCES)) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.className = BUTTON_CLASSES;
    const handler = (): void => {
      for (const code of codes) {
        send({ type: MSG_KEY_EVENT, payload: { type: 'keydown', code } });
      }
      for (let i = codes.length - 1; i >= 0; i--) {
        const code = codes[i];
        if (code === undefined) continue;
        send({ type: MSG_KEY_EVENT, payload: { type: 'keyup', code } });
      }
    };
    btn.addEventListener('click', handler);
    handlers.set(btn, handler);
    containerEl.appendChild(btn);
    buttons.push(btn);
  }

  return () => {
    for (const btn of buttons) {
      const h = handlers.get(btn);
      if (h !== undefined) btn.removeEventListener('click', h);
      btn.remove();
    }
  };
}
