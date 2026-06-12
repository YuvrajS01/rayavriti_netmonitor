import { createContext, useContext } from 'react';

export type EventName =
  | 'metric:update'
  | 'alert:triggered'
  | 'alert:resolved'
  | 'alert:updated'
  | 'device:status'
  | 'bootstrap'
  | 'flow:update'
  | 'capture:packet'
  | 'packet:captured'
  | 'capture:status';

export type Handler = (data: Record<string, unknown>) => void;

export interface SocketContextValue {
  subscribe: (event: EventName, handler: Handler) => () => void;
  emit: (event: string, data?: unknown) => void;
  connected: boolean;
}

export const SocketContext = createContext<SocketContextValue | null>(null);

export function useSocketContext() {
  const ctx = useContext(SocketContext);
  if (!ctx) throw new Error('useSocketContext must be used within SocketProvider');
  return ctx;
}
