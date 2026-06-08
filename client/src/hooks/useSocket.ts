import { useEffect, useRef, useCallback } from 'react';
import { getToken } from '../api/client';

type WSMessage = {
  type: string;
  request_id?: string;
  data: Record<string, unknown>;
};

type SocketHandlers = {
  onMetricUpdate?: (metric: Record<string, unknown>) => void;
  onAlertTriggered?: (alert: Record<string, unknown>) => void;
  onAlertResolved?: (alert: Record<string, unknown>) => void;
  onDeviceStatus?: (status: Record<string, unknown>) => void;
  onBootstrap?: (payload: Record<string, unknown>) => void;
  onFlowUpdate?: (flow: Record<string, unknown>) => void;
  onPacketCaptured?: (packet: Record<string, unknown>) => void;
  onCaptureStatus?: (status: Record<string, unknown>) => void;
};

const WS_URL = import.meta.env.VITE_WS_URL || '/api/v1/ws';
const RECONNECT_BASE_DELAY = 1000;
const RECONNECT_MAX_DELAY = 30000;
const PING_INTERVAL = 30000;

export function useSocket(handlers: SocketHandlers) {
  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef(handlers);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pingTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectAttempts = useRef(0);
  const isCleanClose = useRef(false);

  handlersRef.current = handlers;

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

  const connect = useCallback(() => {
    const token = getToken();
    if (!token) return;

    // Build URL with token as query param (primary auth method for native WS)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const baseWsUrl = import.meta.env.VITE_WS_URL
      ? `${protocol}//${import.meta.env.VITE_WS_URL}`
      : `${protocol}//${host}`;
    const url = new URL(WS_URL, baseWsUrl);
    url.searchParams.set('token', token);

    const ws = new WebSocket(url.toString(), [token]);
    wsRef.current = ws;

    ws.onopen = () => {
      reconnectAttempts.current = 0;
      isCleanClose.current = false;

      // Start ping interval to keep connection alive
      if (pingTimer.current) clearInterval(pingTimer.current);
      pingTimer.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }));
        }
      }, PING_INTERVAL);
    };

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        if (!msg.type || msg.type === 'ping' || msg.type === 'pong') return;

        switch (msg.type) {
          case 'bootstrap':
            handlersRef.current.onBootstrap?.(msg.data);
            break;
          case 'metric:update':
            handlersRef.current.onMetricUpdate?.(msg.data);
            break;
          case 'alert:triggered':
            handlersRef.current.onAlertTriggered?.(msg.data);
            break;
          case 'alert:resolved':
          case 'alert:updated':
            handlersRef.current.onAlertResolved?.(msg.data);
            break;
          case 'device:status':
            handlersRef.current.onDeviceStatus?.(msg.data);
            break;
          case 'flow:update':
            handlersRef.current.onFlowUpdate?.(msg.data);
            break;
          case 'capture:packet':
          case 'packet:captured':
            handlersRef.current.onPacketCaptured?.(msg.data);
            break;
          case 'capture:status':
            handlersRef.current.onCaptureStatus?.(msg.data);
            break;
          default:
            // Unknown event type — ignore
            break;
        }
      } catch {
        // Non-JSON message — ignore
      }
    };

    ws.onclose = (event) => {
      clearTimers();
      wsRef.current = null;

      // Don't reconnect on clean close (code 1000) or auth errors (4001-4003)
      if (isCleanClose.current || (event.code >= 4001 && event.code <= 4003)) {
        return;
      }

      // Exponential backoff reconnect
      const delay = Math.min(
        RECONNECT_BASE_DELAY * Math.pow(2, reconnectAttempts.current),
        RECONNECT_MAX_DELAY,
      );
      reconnectAttempts.current++;

      reconnectTimer.current = setTimeout(() => {
        connect();
      }, delay);
    };

    ws.onerror = () => {
      // onclose will handle reconnection
      ws.close();
    };
  }, [clearTimers]);

  useEffect(() => {
    connect();

    return () => {
      isCleanClose.current = true;
      clearTimers();
      if (wsRef.current) {
        wsRef.current.close(1000, 'component unmount');
        wsRef.current = null;
      }
    };
  }, [connect, clearTimers]);

  const emit = useCallback((event: string, data?: unknown) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: event, data }));
    }
  }, []);

  return { emit, socket: wsRef };
}
