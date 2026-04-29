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

import { type Connection, type ConnectionState } from './connection.js';

const BASE_CLASSES =
  'inline-flex items-center gap-2 px-3 py-1 rounded text-sm font-mono ' +
  'border transition-colors';

const STATE_CLASSES: Record<ConnectionState, string> = {
  connecting: 'border-amber-500 text-amber-700 bg-amber-50',
  connected: 'border-accent text-accent bg-linen',
  reconnecting: 'border-amber-500 text-amber-700 bg-amber-50 animate-pulse',
  disconnected: 'border-sand text-slate bg-paper',
};

const STATE_LABELS: Record<ConnectionState, string> = {
  connecting: 'connecting',
  connected: 'connected',
  reconnecting: 'reconnecting',
  disconnected: 'disconnected',
};

export function attachStatusIndicator(
  el: HTMLElement,
  conn: Connection,
): () => void {
  const render = (state: ConnectionState): void => {
    el.className = `${BASE_CLASSES} ${STATE_CLASSES[state]}`;
    el.textContent = STATE_LABELS[state];
  };
  render(conn.state);
  const handler = (ev: Event): void => {
    if (!(ev instanceof CustomEvent)) return;
    const detail = ev.detail as ConnectionState;
    render(detail);
  };
  conn.addEventListener('state', handler);
  return () => {
    conn.removeEventListener('state', handler);
  };
}
