export type WSEventType = 'text_delta' | 'tool_call' | 'tool_result' | 'done' | 'error';

export interface WSEvent {
  type: WSEventType;
  content?: string;
  name?: string;
  args?: Record<string, any>;
}

export type WSEventHandler = (event: WSEvent) => void;

export class ForgeWebSocket {
  private ws: WebSocket | null = null;
  private sessionId: string;
  private handler: WSEventHandler;
  private reconnectTimer: number | null = null;

  constructor(sessionId: string, handler: WSEventHandler) {
    this.sessionId = sessionId;
    this.handler = handler;
  }

  connect(): void {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/api/sessions/${this.sessionId}/ws`;

    this.ws = new WebSocket(url);

    this.ws.onmessage = (event) => {
      try {
        const data: WSEvent = JSON.parse(event.data);
        this.handler(data);
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err);
      }
    };

    this.ws.onclose = (event) => {
      if (!event.wasClean) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      // onclose will fire after this
    };
  }

  send(content: string): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'message', content }));
    }
  }

  close(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
  }

  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer !== null) return;
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, 2000);
  }
}
