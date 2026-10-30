import {
  isAuthRejection, refreshAccessToken, setAccessToken, setSessionExpiredHandler, v1,
} from './http';
import { queryClient } from './queryClient';
import { sessionStore, type SessionUser } from '../store/session';

/**
 * Session controller. The access token lives only in memory (http.ts); the
 * HttpOnly refresh cookie restores it on page load. Nothing auth-related is
 * persisted in localStorage.
 */

// Keys written by earlier versions; removed so stale tokens don't linger.
const LEGACY_STORAGE_KEYS = ['netmonitor_token', 'netmonitor_refresh_token', 'netmonitor_user'];

const CHANNEL_NAME = 'netmonitor-auth';
type AuthMessage = { type: 'logout' } | { type: 'login' };

const channel: BroadcastChannel | null =
  typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel(CHANNEL_NAME) : null;

/** Drop the in-memory session and cached data for this tab. */
function clearLocalSession() {
  setAccessToken(null);
  queryClient.clear();
  sessionStore.setUnauthenticated();
}

setSessionExpiredHandler(clearLocalSession);

channel?.addEventListener('message', (event: MessageEvent<AuthMessage>) => {
  if (event.data?.type === 'logout') {
    clearLocalSession();
  } else if (event.data?.type === 'login' && sessionStore.getState().status !== 'authenticated') {
    void restoreSession();
  }
});

/** Called once on app start: exchange the refresh cookie for a session. */
export async function restoreSession(): Promise<void> {
  for (const key of LEGACY_STORAGE_KEYS) {
    try { localStorage.removeItem(key); } catch { /* storage unavailable */ }
  }
  sessionStore.setRestoring();
  try {
    const { user } = await refreshAccessToken();
    if (user) sessionStore.setAuthenticated(user);
    else sessionStore.setUnauthenticated();
  } catch (error) {
    // A rejected refresh token already ended the session via the handler;
    // anything else (server down, timeout) is retryable, not a logout.
    if (isAuthRejection(error)) sessionStore.setUnauthenticated();
    else sessionStore.setUnreachable();
  }
}

interface LoginResponse {
  token: string;
  user: SessionUser;
}

export async function login(username: string, password: string): Promise<LoginResponse> {
  const r = await v1.post('/auth/login', { username, password });
  const body = r.data as Record<string, unknown>;
  const record = (body?.data !== undefined ? body.data : body) as Record<string, unknown>;
  const token = (record?.accessToken || record?.token) as string;
  const user = record?.user as SessionUser;
  setAccessToken(token);
  queryClient.clear();
  sessionStore.setAuthenticated(user);
  channel?.postMessage({ type: 'login' } satisfies AuthMessage);
  return { token, user };
}

/**
 * Revoke the refresh token server-side (clears both cookies), then end the
 * session here and in every other open tab. Resolves even if the request
 * fails; returns whether the server confirmed the logout.
 */
export async function logout(): Promise<boolean> {
  let confirmed = true;
  try {
    await v1.post('/auth/logout', {});
  } catch {
    confirmed = false;
  }
  clearLocalSession();
  channel?.postMessage({ type: 'logout' } satisfies AuthMessage);
  return confirmed;
}
