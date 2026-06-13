import { useEffect, useRef } from 'react';
import { useSocketContext } from './socketContext';

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

const EVENT_MAP: Record<keyof SocketHandlers, string> = {
  onMetricUpdate: 'metric:update',
  onAlertTriggered: 'alert:triggered',
  onAlertResolved: 'alert:resolved',
  onDeviceStatus: 'device:status',
  onBootstrap: 'bootstrap',
  onFlowUpdate: 'flow:update',
  onPacketCaptured: 'capture:packet',
  onCaptureStatus: 'capture:status',
};

export function useSocketEvents(handlers: SocketHandlers) {
  const { subscribe, emit } = useSocketContext();
  const handlersRef = useRef(handlers);

  useEffect(() => {
    handlersRef.current = handlers;
  }, [handlers]);

  useEffect(() => {
    const cleanups: (() => void)[] = [];

    for (const [prop, event] of Object.entries(EVENT_MAP) as [keyof SocketHandlers, string][]) {
      if (handlersRef.current[prop]) {
        const unsub = subscribe(event as never, (data: Record<string, unknown>) => {
          handlersRef.current[prop]?.(data);
        });
        cleanups.push(unsub);
      }
    }

    return () => cleanups.forEach((fn) => fn());
  }, [subscribe]);

  return { emit };
}
