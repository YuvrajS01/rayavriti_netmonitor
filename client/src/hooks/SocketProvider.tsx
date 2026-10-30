import { useEffect, useRef, useCallback, useMemo, useState } from 'react';
import { useSelector } from 'react-redux';
import { getToken } from '../api/client';
import { SocketContext, SocketStatusContext } from './socketContext';
import { resolveWebSocketUrl } from './socketUrl';
import { BATCH_WINDOW_MS, reconnectDelay } from './socketBackoff';
import { parseWsMessage, type WsEventName, type WsHandler, type WsMessage } from '../api/wsEvents';
import type { RootState } from '../store';

const PING_INTERVAL = 30000;
const BATCHED_EVENTS: ReadonlySet<WsEventName> = new Set<WsEventName>(['metric:update']);

type AnyHandler = (data: unknown) => void;

export function SocketProvider({ children }: { children: React.ReactNode }) {
  const handlersRef = useRef<Map<WsEventName, Set<AnyHandler>>>(new Map());
  const [connected, setConnected] = useState(false);
  const isAuthenticated = useSelector((s: RootState) => s.auth.isAuthenticated);

  const dispatch = useCallback((msg: WsMessage) => {
    const handlers = handlersRef.current.get(msg.type);
    if (!handlers) return;
    for (const handler of [...handlers]) {
      handler(msg.data);
    }
  }, []);

  useEffect(() => {
    if (!isAuthenticated) return;

    // All connection state is local to this effect run, so a socket from a
    // previous run (StrictMode double-mount, logout/login) can never touch
    // the live one. The `ws !== socket` guards cover sockets superseded
    // within the same run.
    let disposed = false;
    let ws: WebSocket | null = null;
    let attempts = 0;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let pingTimer: ReturnType<typeof setInterval> | null = null;
    let flushTimer: ReturnType<typeof setTimeout> | null = null;
    let queue: WsMessage[] = [];

    const flush = () => {
      if (flushTimer) {
        clearTimeout(flushTimer);
        flushTimer = null;
      }
      const batch = queue;
      queue = [];
      // Delivered in one synchronous pass so React batches the resulting
      // state updates into a single render.
      for (const msg of batch) dispatch(msg);
    };

    const stopPing = () => {
      if (pingTimer) {
        clearInterval(pingTimer);
        pingTimer = null;
      }
    };

    const scheduleReconnect = () => {
      if (disposed || reconnectTimer) return;
      const delay = reconnectDelay(attempts);
      attempts += 1;
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
      }, delay);
    };

    function connect() {
      if (disposed) return;
      const token = getToken();
      const url = resolveWebSocketUrl();
      // The access token is offered as a subprotocol; the backend falls back
      // to the HttpOnly access cookie when none is sent.
      const socket = token ? new WebSocket(url, [token]) : new WebSocket(url);
      ws = socket;

      socket.onopen = () => {
        if (ws !== socket) return;
        attempts = 0;
        setConnected(true);
        stopPing();
        pingTimer = setInterval(() => {
          if (socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'ping' }));
          }
        }, PING_INTERVAL);
      };

      socket.onmessage = (event: MessageEvent) => {
        if (ws !== socket) return;
        const msg = parseWsMessage(event.data);
        if (!msg) return;
        if (BATCHED_EVENTS.has(msg.type)) {
          queue.push(msg);
          if (!flushTimer) flushTimer = setTimeout(flush, BATCH_WINDOW_MS);
          return;
        }
        // Deliver anything already buffered first to preserve ordering.
        if (queue.length) flush();
        dispatch(msg);
      };

      socket.onclose = () => {
        if (ws !== socket) return;
        ws = null;
        stopPing();
        flush();
        setConnected(false);
        scheduleReconnect();
      };

      socket.onerror = () => {
        socket.close();
      };
    }

    const reconnectNow = () => {
      if (disposed) return;
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      attempts = 0;
      connect();
    };

    const handleVisibility = () => {
      if (document.visibilityState === 'visible') reconnectNow();
    };

    window.addEventListener('online', reconnectNow);
    document.addEventListener('visibilitychange', handleVisibility);
    connect();

    return () => {
      disposed = true;
      window.removeEventListener('online', reconnectNow);
      document.removeEventListener('visibilitychange', handleVisibility);
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (flushTimer) clearTimeout(flushTimer);
      stopPing();
      queue = [];
      const socket = ws;
      ws = null;
      if (socket) socket.close(1000, 'provider unmount');
      setConnected(false);
    };
  }, [isAuthenticated, dispatch]);

  const subscribe = useCallback(<K extends WsEventName>(event: K, handler: WsHandler<K>) => {
    let handlers = handlersRef.current.get(event);
    if (!handlers) {
      handlers = new Set();
      handlersRef.current.set(event, handlers);
    }
    const set = handlers;
    const fn = handler as AnyHandler;
    set.add(fn);

    return () => {
      set.delete(fn);
      if (set.size === 0 && handlersRef.current.get(event) === set) {
        handlersRef.current.delete(event);
      }
    };
  }, []);

  const value = useMemo(() => ({ subscribe }), [subscribe]);

  return (
    <SocketContext.Provider value={value}>
      <SocketStatusContext.Provider value={connected}>
        {children}
      </SocketStatusContext.Provider>
    </SocketContext.Provider>
  );
}
