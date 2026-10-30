import axios, { isAxiosError, type AxiosError, type InternalAxiosRequestConfig } from 'axios';
import type { SessionUser } from '../store/session';

const API_BASE = import.meta.env.VITE_API_V1_URL || '/api/v1';

export const v1 = axios.create({
  baseURL: API_BASE,
  timeout: 30_000,
  withCredentials: true,
});

/**
 * Refresh calls go through their own instance so the 401 interceptor below
 * can never recurse into itself. The refresh token travels in the HttpOnly
 * cookie (scoped to /api/v1/auth), never in JS.
 */
export const authClient = axios.create({
  baseURL: API_BASE,
  timeout: 10_000,
  withCredentials: true,
});

// ── Access token (memory only) ───────────────────────────────────────────

let accessToken: string | null = null;

export const setAccessToken = (token: string | null) => {
  accessToken = token && token !== 'undefined' ? token : null;
};

export const getAccessToken = () => accessToken;

const EXPIRY_SKEW_MS = 30_000;

/** Reads `exp` from a JWT payload (no verification — only used for scheduling). */
export function tokenExpiresAt(token: string): number | null {
  const payload = token.split('.')[1];
  if (!payload) return null;
  try {
    const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'));
    const exp = (JSON.parse(json) as { exp?: unknown }).exp;
    return typeof exp === 'number' ? exp * 1000 : null;
  } catch {
    return null;
  }
}

// ── Session expiry hook ──────────────────────────────────────────────────

let sessionExpiredHandler: () => void = () => {};

/** Registered by api/auth.ts; invoked when the refresh token is rejected. */
export function setSessionExpiredHandler(handler: () => void) {
  sessionExpiredHandler = handler;
}

/** True when the server definitively rejected the credentials (vs. a network/5xx failure). */
export function isAuthRejection(error: unknown): boolean {
  if (!isAxiosError(error)) return false;
  const status = error.response?.status;
  return status === 401 || status === 403;
}

// ── Single-flight refresh ────────────────────────────────────────────────

export interface RefreshResult {
  accessToken: string | null;
  user: SessionUser | null;
}

function extractSession(raw: unknown): RefreshResult {
  const body = raw as Record<string, unknown> | null;
  const data = (body?.data !== undefined ? body.data : body) as Record<string, unknown> | null;
  const token = (data?.accessToken || data?.token) as string | undefined;
  return { accessToken: token ?? null, user: (data?.user as SessionUser | undefined) ?? null };
}

const REFRESH_LOCK = 'netmonitor-auth-refresh';

async function performRefresh(): Promise<RefreshResult> {
  const run = async () => {
    const { data } = await authClient.post<unknown>('/auth/refresh', {});
    return extractSession(data);
  };
  // The backend rotates refresh tokens and treats concurrent reuse as theft
  // (revoking the whole family). Serialise refreshes across tabs where the
  // Web Locks API is available; within a tab, single-flight covers it.
  if (typeof navigator !== 'undefined' && navigator.locks?.request) {
    return navigator.locks.request(REFRESH_LOCK, run);
  }
  return run();
}

let refreshPromise: Promise<RefreshResult> | null = null;

/**
 * Exchange the refresh cookie for a new access token. Concurrent callers
 * share one in-flight request. If the server rejects the refresh token the
 * session-expired handler runs before the promise rejects.
 */
export function refreshAccessToken(): Promise<RefreshResult> {
  if (!refreshPromise) {
    refreshPromise = performRefresh()
      .then((result) => {
        setAccessToken(result.accessToken);
        return result;
      })
      .catch((error: unknown) => {
        if (isAuthRejection(error)) {
          setAccessToken(null);
          sessionExpiredHandler();
        }
        throw error;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

/**
 * An access token valid for at least the next 30s, refreshing if needed.
 * Returns null when no token can be obtained (callers fall back to the
 * access cookie).
 */
export async function getFreshAccessToken(): Promise<string | null> {
  const expiresAt = accessToken ? tokenExpiresAt(accessToken) : null;
  if (accessToken && (expiresAt === null || expiresAt - Date.now() > EXPIRY_SKEW_MS)) {
    return accessToken;
  }
  try {
    return (await refreshAccessToken()).accessToken;
  } catch {
    return null;
  }
}

// ── Interceptors ─────────────────────────────────────────────────────────

type RetriableConfig = InternalAxiosRequestConfig & { _retry?: boolean };

/** Endpoints whose 401 means "bad credentials", not "access token expired". */
const NO_REFRESH_PATHS = ['/auth/login', '/auth/refresh', '/auth/logout'];

const bearer = (token: string) => `Bearer ${token}`;

v1.interceptors.request.use((config) => {
  if (accessToken) config.headers.Authorization = bearer(accessToken);
  return config;
});

let maintenanceHandler: () => void = () => {
  window.location.href = '/system-maintenance';
};

/** Lets the router take over the maintenance redirect (avoids a full reload). */
export function setMaintenanceHandler(handler: () => void) {
  maintenanceHandler = handler;
}

async function handleResponseError(error: AxiosError) {
  const original = error.config as RetriableConfig | undefined;
  const responseBody = error.response?.data as { error?: { message?: string } } | { error?: string } | undefined;
  const message = typeof responseBody?.error === 'string' ? responseBody.error : responseBody?.error?.message;
  if (error.response?.status === 503 && message === 'System is under maintenance') {
    maintenanceHandler();
    return Promise.reject(error);
  }

  if (
    !original ||
    error.response?.status !== 401 ||
    original._retry ||
    NO_REFRESH_PATHS.some((path) => original.url?.includes(path))
  ) {
    return Promise.reject(error);
  }

  // Mark before awaiting so a request can only ever be retried once, whether
  // it triggered the refresh or queued behind someone else's.
  original._retry = true;

  // If the token already changed since this request was sent (another
  // request refreshed meanwhile), just replay with the current one.
  const sent = original.headers.Authorization;
  let token = accessToken;
  if (!token || sent === bearer(token)) {
    try {
      token = (await refreshAccessToken()).accessToken;
    } catch {
      return Promise.reject(error);
    }
  }

  if (token) original.headers.Authorization = bearer(token);
  else delete original.headers.Authorization;
  // Replay through v1 so the request interceptor and this handler apply.
  return v1(original);
}

v1.interceptors.response.use((res) => res, handleResponseError);

// ── Response helpers ─────────────────────────────────────────────────────

/** Options accepted by every api function; `signal` cancels the request. */
export interface RequestOptions {
  signal?: AbortSignal;
}

/** Trigger a browser download for a blob fetched through the api layer. */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function unwrapGoResponse<T>(raw: unknown): T {
  const body = raw as Record<string, unknown>;
  const data = body?.data !== undefined ? body.data : body;
  if (data && typeof data === 'object' && 'alerts' in data && 'total' in data) {
    return (data as Record<string, unknown>).alerts as T;
  }
  return data as T;
}

export function wrap<T>(raw: unknown): { data: T; success: boolean } {
  return { data: unwrapGoResponse<T>(raw), success: true };
}
