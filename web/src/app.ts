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

import { attachChannelSelector } from './channel-selector.js';
import { Connection } from './connection.js';
import { attachControlSequences } from './control-sequences.js';
import { attachKeyboard } from './keyboard.js';
import { attachStatusIndicator } from './status-indicator.js';

function init(): void {
  const video = document.getElementById('video');
  const selector = document.getElementById('channel-selector');
  const sequences = document.getElementById('control-sequences');
  const status = document.getElementById('status');

  if (!(video instanceof HTMLImageElement)) {
    throw new Error('expected #video to be an <img>');
  }
  if (!(selector instanceof HTMLSelectElement)) {
    throw new Error('expected #channel-selector to be a <select>');
  }
  if (!(sequences instanceof HTMLElement)) {
    throw new Error('expected #control-sequences container');
  }
  if (!(status instanceof HTMLElement)) {
    throw new Error('expected #status container');
  }

  const conn = new Connection();
  conn.addEventListener('mjpeg-url', (ev: Event) => {
    if (!(ev instanceof CustomEvent)) return;
    video.src = ev.detail as string;
  });

  attachKeyboard(window, (env) => conn.send(env));
  attachChannelSelector(selector, (env) => conn.send(env));
  attachControlSequences(sequences, (env) => conn.send(env));
  attachStatusIndicator(status, conn);

  conn.start();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
