import { useSyncExternalStore } from 'react';

export interface SessionUser {
  id: number;
  username: string;
  role: string;
  permissions?: string[];
}

/**
 * - `restoring`: app start, waiting for the cookie-based refresh.
 * - `unreachable`: the restore attempt failed for a non-auth reason
 *   (server down); the user is not logged out, just asked to retry.
 */
export type SessionStatus = 'restoring' | 'authenticated' | 'unauthenticated' | 'unreachable';

export interface SessionState {
  status: SessionStatus;
  user: SessionUser | null;
}

type Listener = () => void;

/**
 * Minimal external store for the signed-in session. It replaces the Redux
 * auth slice: the only global client state is "who is logged in", and
 * keeping it outside React lets the HTTP layer end the session directly.
 */
function createSessionStore(initial: SessionState) {
  let state = initial;
  const listeners = new Set<Listener>();

  const set = (next: SessionState) => {
    state = next;
    listeners.forEach((fn) => fn());
  };

  return {
    getState: () => state,
    subscribe: (listener: Listener) => {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    setRestoring: () => set({ status: 'restoring', user: state.user }),
    setAuthenticated: (user: SessionUser) => set({ status: 'authenticated', user }),
    setUnauthenticated: () => set({ status: 'unauthenticated', user: null }),
    setUnreachable: () => set({ status: 'unreachable', user: null }),
  };
}

export const sessionStore = createSessionStore({ status: 'restoring', user: null });

/** Subscribe to a slice of the session; the selector must return a stable value. */
function useSessionSelector<T>(selector: (state: SessionState) => T): T {
  return useSyncExternalStore(sessionStore.subscribe, () => selector(sessionStore.getState()));
}

export function useSessionStatus(): SessionStatus {
  return useSessionSelector((s) => s.status);
}

export function useSessionUser(): SessionUser | null {
  return useSessionSelector((s) => s.user);
}

export function useIsAuthenticated(): boolean {
  return useSessionSelector((s) => s.status === 'authenticated');
}
