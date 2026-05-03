import { useEffect, useRef, useCallback } from 'react';
import { io, type Socket } from 'socket.io-client';
import { getToken } from '../api/client';

export function useSocket(handlers: {
  onMetricUpdate?: (metric: Record<string, unknown>) => void;
  onAlertTriggered?: (alert: Record<string, unknown>) => void;
  onAlertResolved?: (alert: Record<string, unknown>) => void;
  onDeviceStatus?: (status: Record<string, unknown>) => void;
  onBootstrap?: (payload: Record<string, unknown>) => void;
  onFlowUpdate?: (flow: Record<string, unknown>) => void;
  onPacketCaptured?: (packet: Record<string, unknown>) => void;
  onCaptureStatus?: (status: Record<string, unknown>) => void;
}) {
  const socketRef = useRef<Socket | null>(null);
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const socket = io({ auth: { token } });
    socketRef.current = socket;

    socket.on('bootstrap', (payload) => handlersRef.current.onBootstrap?.(payload));
    socket.on('metric:update', (metric) => handlersRef.current.onMetricUpdate?.(metric));
    socket.on('alert:triggered', (alert) => handlersRef.current.onAlertTriggered?.(alert));
    socket.on('alert:resolved', (alert) => handlersRef.current.onAlertResolved?.(alert));
    socket.on('device:status', (status) => handlersRef.current.onDeviceStatus?.(status));
    socket.on('flow:update', (flow) => handlersRef.current.onFlowUpdate?.(flow));
    socket.on('packet:captured', (packet) => handlersRef.current.onPacketCaptured?.(packet));
    socket.on('capture:status', (status) => handlersRef.current.onCaptureStatus?.(status));

    return () => { socket.disconnect(); };
  }, []);

  const emit = useCallback((event: string, data?: unknown) => {
    socketRef.current?.emit(event, data);
  }, []);

  return { emit, socket: socketRef };
}
