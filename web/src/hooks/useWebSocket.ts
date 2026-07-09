import { useRef, useEffect, useCallback } from 'react';
import type { WSMessage } from '../lib/types';

interface UseWebSocketOptions {
  sessionId: string;
  onMessage: (msg: WSMessage) => void;
  onOpen?: () => void; onClose?: () => void;
  enabled?: boolean;
}

export function useWebSocket({ sessionId, onMessage, onOpen, onClose, enabled = true }: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef(0);

  const connect = useCallback(() => {
    if (!enabled || !sessionId) return;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?session_id=${encodeURIComponent(sessionId)}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;
    ws.onopen = () => { reconnectRef.current = 0; onOpen?.(); };
    ws.onmessage = (event) => {
      try { const msg: WSMessage = JSON.parse(event.data); onMessage(msg); } catch {}
    };
    ws.onclose = () => {
      onClose?.();
      if (reconnectRef.current < 5) {
        const delay = Math.min(1000 * 2 ** reconnectRef.current, 10000);
        reconnectRef.current++;
        setTimeout(connect, delay);
      }
    };
    ws.onerror = () => {};
  }, [sessionId, enabled, onMessage, onOpen, onClose]);

  useEffect(() => { connect(); return () => { reconnectRef.current = 99; wsRef.current?.close(); }; }, [connect]);
  return wsRef;
}
