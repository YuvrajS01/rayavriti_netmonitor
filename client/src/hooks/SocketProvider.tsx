import { useEffect, useRef, useCallback, useState } from 'react';
import { useSelector } from 'react-redux';
import { getToken } from '../api/client';
import { SocketContext, type EventName, type Handler } from './socketContext';
import { resolveWebSocketUrl } from './socketUrl';
import type { RootState } from '../store';

const RECONNECT_BASE_DELAY = 1000;
const RECONNECT_MAX_DELAY = 30000;
const PING_INTERVAL = 30000;

export function SocketProvider({ children }: { children: React.ReactNode }) {
  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef<Map<EventName, Set<Handler>>>(new Map());
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pingTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectAttempts = useRef(0);
  const isCleanClose = useRef(false);
  const [connected, setConnected] = useState(false);
  const isAuthenticated = useSelector((s: RootState) => s.auth.isAuthenticated);

  const clearTimers = useCallback(() => {
    if (reconnectTimer.current) {
      clearTimeout(reconnectTimer.current);
      reconnectTimer.current = null;
    }
    if (pingTimer.current) {
      clearInterval(pingTimer.current);
      pingTimer.current = null;
    }
  }, []);

  const connectRef = useRef<() => void>(() => {});

  const connect = useCallback(() => {
    if (!isAuthenticated) {
      isCleanClose.current = true;
      clearTimers();
      if (wsRef.current) {
        wsRef.current.close(1000, 'not authenticated');
        wsRef.current = null;
      }
      return;
    }

    const token = getToken();

    const url = resolveWebSocketUrl();
    const isSameOrigin = new URL(url).host === window.location.host;
    const ws = token && !isSameOrigin ? new WebSocket(url, [token]) : new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      reconnectAttempts.current = 0;
      isCleanClose.current = false;
      setConnected(true);

      if (pingTimer.current) clearInterval(pingTimer.current);
      pingTimer.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }));
        }
      }, PING_INTERVAL);
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as { type: string; data: Record<string, unknown> };
        if (!msg.type || msg.type === 'ping' || msg.type === 'pong') return;

        const handlers = handlersRef.current.get(msg.type as EventName);
        if (handlers) {
          for (const handler of handlers) {
            handler(msg.data);
          }
        }
      } catch {
        // Non-JSON message — ignore
      }
    };

    ws.onclose = (event) => {
      clearTimers();
      wsRef.current = null;
      setConnected(false);

      if (isCleanClose.current || (event.code >= 4001 && event.code <= 4003)) {
        return;
      }

      const delay = Math.min(
        RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts.current),
        RECONNECT_MAX_DELAY,
      );
      reconnectAttempts.current++;

      reconnectTimer.current = setTimeout(() => {
        connectRef.current();
      }, delay);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [clearTimers, isAuthenticated]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    connect();

    const handleBeforeUnload = () => {
      isCleanClose.current = true;
      clearTimers();
      if (wsRef.current) {
        wsRef.current.close(1000, 'beforeunload');
        wsRef.current = null;
      }
    };
    window.addEventListener('beforeunload', handleBeforeUnload);

    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
      isCleanClose.current = true;
      clearTimers();
      if (wsRef.current) {
        wsRef.current.close(1000, 'provider unmount');
        wsRef.current = null;
      }
    };
  }, [connect, clearTimers]);

  const subscribe = useCallback((event: EventName, handler: Handler) => {
    let handlers = handlersRef.current.get(event);
    if (!handlers) {
      handlers = new Set();
      handlersRef.current.set(event, handlers);
    }
    handlers.add(handler);

    return () => {
      handlers!.delete(handler);
      if (handlers!.size === 0) {
        handlersRef.current.delete(event);
      }
    };
  }, []);

  const emit = useCallback((event: string, data?: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: event, data }));
    }
  }, []);

  return (
    <SocketContext.Provider value={{ subscribe, emit, connected }}>
      {children}
    </SocketContext.Provider>
  );
}
