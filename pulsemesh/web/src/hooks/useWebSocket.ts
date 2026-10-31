import { useEffect, useRef, useState, useCallback } from 'react';
import type { TopologyData } from '../types';

interface UseWebSocketReturn {
  data: TopologyData | null;
  connected: boolean;
}

export function useWebSocket(): UseWebSocketReturn {
  const [data, setData] = useState<TopologyData | null>(null);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const backoffRef = useRef(1000);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      backoffRef.current = 1000;
    };

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data) as TopologyData;
        setData(parsed);
      } catch {
        console.warn('[PulseMesh] Failed to parse WebSocket message');
      }
    };

    ws.onclose = () => {
      setConnected(false);
      wsRef.current = null;
      const delay = backoffRef.current;
      backoffRef.current = Math.min(delay * 2, 10000);
      reconnectTimerRef.current = setTimeout(connect, delay);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, []);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { data, connected };
}
