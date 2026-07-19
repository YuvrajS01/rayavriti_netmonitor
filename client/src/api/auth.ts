import { getAccessToken, setAccessToken, v1 } from './http';

const TOKEN_STORAGE_KEY = 'netmonitor_token';

interface LoginResponse {
  token: string;
  refreshToken: string;
  user: { id: number; username: string; role: string; permissions?: string[] };
}

// Rehydrate the in-memory access token on module load so that the WebSocket
// connection can authenticate immediately after a page reload, without
// waiting for a fresh login. The persisted user in localStorage is what
// drives `isAuthenticated`, so the token must be restored alongside it.
if (typeof window !== 'undefined') {
  const persisted = window.localStorage.getItem(TOKEN_STORAGE_KEY);
  if (persisted) setAccessToken(persisted);
}

export const login = (username: string, password: string) =>
  v1.post('/auth/login', { username, password }).then((r): { success: boolean; data: LoginResponse } => {
    const raw = r.data;
    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    const record = data as Record<string, unknown>;
    const token = (record?.accessToken || record?.token) as string;
    const refreshToken = record?.refreshToken as string;
    const user = record?.user as LoginResponse['user'];
    setAccessToken(token);
    localStorage.setItem(TOKEN_STORAGE_KEY, token);
    localStorage.setItem('netmonitor_user', JSON.stringify(user));
    return { success: true, data: { token, refreshToken, user } };
  });

export const logout = () =>
  v1.post('/auth/logout').finally(() => {
    setAccessToken(null);
    localStorage.removeItem(TOKEN_STORAGE_KEY);
    localStorage.removeItem('netmonitor_user');
  });

export const getToken = getAccessToken;
