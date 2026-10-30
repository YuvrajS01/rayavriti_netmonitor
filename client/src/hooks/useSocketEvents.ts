import { useEffect, useRef } from 'react';
import { useSocketContext } from './socketContext';
import type { WsEventName, WsHandler } from '../api/wsEvents';

const EVENT_MAP = {
  onMetricUpdate: 'metric:update',
  onAlertTriggered: 'alert:triggered',
  onAlertUpdated: 'alert:updated',
  onAlertResolved: 'alert:resolved',
  onDeviceStatus: 'device:status',
  onBootstrap: 'bootstrap',
  onFlowUpdate: 'flow:update',
  onPacketCaptured: 'capture:packet',
  onCaptureStatus: 'capture:status',
  onPortsScanned: 'ports:scanned',
  onRemoteStatus: 'remote:status',
  onRemoteMetrics: 'remote:metrics',
} as const satisfies Record<string, WsEventName>;

type HandlerProp = keyof typeof EVENT_MAP;

export type SocketHandlers = {
  [P in HandlerProp]?: WsHandler<(typeof EVENT_MAP)[P]>;
};

const HANDLER_PROPS = Object.keys(EVENT_MAP) as HandlerProp[];

/**
 * Subscribe to typed WebSocket events for the lifetime of the component.
 * Handlers may change identity on every render; the latest one is always
 * invoked without resubscribing.
 */
export function useSocketEvents(handlers: SocketHandlers) {
  const { subscribe } = useSocketContext();
  const handlersRef = useRef(handlers);

  useEffect(() => {
    handlersRef.current = handlers;
  });

  // Resubscribe only when the *set* of handled events changes.
  const activeKey = HANDLER_PROPS.filter((prop) => handlers[prop]).join(',');

  useEffect(() => {
    const props = activeKey ? (activeKey.split(',') as HandlerProp[]) : [];
    const cleanups = props.map((prop) =>
      subscribe(EVENT_MAP[prop], (data: unknown) => {
        (handlersRef.current[prop] as ((payload: unknown) => void) | undefined)?.(data);
      }),
    );
    return () => cleanups.forEach((fn) => fn());
  }, [subscribe, activeKey]);
}
