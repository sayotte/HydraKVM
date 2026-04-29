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
  type ConnectResponse,
  type Envelope,
  type MJPEGURLPayload,
  MSG_MJPEG_URL,
} from './protocol.js';

const RECONNECT_BASE_DELAY_MS = 500;
const RECONNECT_MAX_DELAY_MS = 30_000;
const RECONNECT_JITTER = 0.25;

export type ConnectionState =
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'disconnected';

export class Connection extends EventTarget {
  private ws: WebSocket | null = null;
  private stopping = false;
  private attempt = 0;
  private queue: string[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private currentState: ConnectionState = 'disconnected';

  get state(): ConnectionState {
    return this.currentState;
  }

  start(): void {
    this.stopping = false;
    void this.connect();
  }

  stop(): void {
    this.stopping = true;
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws !== null) {
      try {
        this.ws.close(1000, 'client stop');
      } catch {
        /* noop */
      }
      this.ws = null;
    }
    this.setState('disconnected');
  }

  send<T>(env: Envelope<T>): void {
    const payload = JSON.stringify(env);
    if (this.ws !== null && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(payload);
      return;
    }
    this.queue.push(payload);
  }

  private setState(next: ConnectionState): void {
    if (this.currentState === next) return;
    this.currentState = next;
    this.dispatchEvent(new CustomEvent('state', { detail: next }));
  }

  private async connect(): Promise<void> {
    this.setState(this.attempt === 0 ? 'connecting' : 'reconnecting');
    let wsUrl: string;
    try {
      const resp = await fetch('/api/connect', { method: 'GET' });
      if (!resp.ok) {
        throw new Error(`/api/connect HTTP ${resp.status}`);
      }
      const body = (await resp.json()) as ConnectResponse;
      wsUrl = body.ws_url;
    } catch (err: unknown) {
      console.warn('connect bootstrap failed:', err);
      this.scheduleReconnect();
      return;
    }

    const url = new URL(wsUrl, window.location.href);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(url.toString());
    this.ws = ws;

    ws.addEventListener('open', () => {
      this.attempt = 0;
      this.setState('connected');
      this.flush();
      this.dispatchEvent(new Event('open'));
    });

    ws.addEventListener('message', (ev) => {
      this.handleMessage(ev.data);
    });

    ws.addEventListener('close', (ev) => {
      this.ws = null;
      this.dispatchEvent(new CloseEvent('close', ev));
      if (this.stopping) {
        this.setState('disconnected');
        return;
      }
      this.scheduleReconnect();
    });

    ws.addEventListener('error', () => {
      // Errors precede 'close'; let the close handler drive reconnect.
    });
  }

  private flush(): void {
    if (this.ws === null || this.ws.readyState !== WebSocket.OPEN) return;
    while (this.queue.length > 0) {
      const next = this.queue.shift();
      if (next !== undefined) this.ws.send(next);
    }
  }

  private handleMessage(data: unknown): void {
    if (typeof data !== 'string') return;
    let env: Envelope;
    try {
      env = JSON.parse(data) as Envelope;
    } catch {
      return;
    }
    if (env.type === MSG_MJPEG_URL) {
      const payload = env.payload as MJPEGURLPayload | undefined;
      if (payload !== undefined && typeof payload.url === 'string') {
        this.dispatchEvent(
          new CustomEvent('mjpeg-url', { detail: payload.url }),
        );
        return;
      }
    }
    this.dispatchEvent(new CustomEvent('message', { detail: env }));
  }

  private scheduleReconnect(): void {
    if (this.stopping) return;
    this.setState('reconnecting');
    const exp = Math.min(
      RECONNECT_MAX_DELAY_MS,
      RECONNECT_BASE_DELAY_MS * 2 ** this.attempt,
    );
    const jitterFactor =
      1 - RECONNECT_JITTER + Math.random() * 2 * RECONNECT_JITTER;
    const delay = Math.round(exp * jitterFactor);
    this.attempt += 1;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, delay);
  }
}
