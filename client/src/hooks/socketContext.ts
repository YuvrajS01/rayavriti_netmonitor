import { createContext, useContext } from 'react';
import type { WsEventName, WsHandler } from '../api/wsEvents';

export type { WsEventName as EventName } from '../api/wsEvents';

export interface SocketContextValue {
  subscribe: <K extends WsEventName>(event: K, handler: WsHandler<K>) => () => void;
}

export const SocketContext = createContext<SocketContextValue | null>(null);

/**
 * Connection state lives in its own context so that subscribers (which only
 * need the stable `subscribe`) do not re-render on every connect/disconnect.
 */
export const SocketStatusContext = createContext<boolean>(false);

export function useSocketContext() {
  const ctx = useContext(SocketContext);
  if (!ctx) throw new Error('useSocketContext must be used within SocketProvider');
  return ctx;
}

export function useSocketConnected() {
  return useContext(SocketStatusContext);
}
