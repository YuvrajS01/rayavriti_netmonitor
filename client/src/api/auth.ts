import { api } from './http';

interface LoginResponse {
  token: string;
  refreshToken: string;
  user: { id: number; username: string; role: string; permissions?: string[] };
}

export const login = (username: string, password: string) =>
  api.post('/auth/login', { username, password }).then((r): { success: boolean; data: LoginResponse } => {
    const raw = r.data;
    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    const record = data as Record<string, unknown>;
    const token = (record?.accessToken || record?.token) as string;
    const refreshToken = record?.refreshToken as string;
    const user = record?.user as LoginResponse['user'];
    localStorage.setItem('netmonitor_token', token);
    localStorage.setItem('netmonitor_refresh_token', refreshToken);
    localStorage.setItem('netmonitor_user', JSON.stringify(user));
    return { success: true, data: { token, refreshToken, user } };
  });

export const logout = () =>
  api.post('/auth/logout').finally(() => {
    localStorage.removeItem('netmonitor_token');
    localStorage.removeItem('netmonitor_refresh_token');
    localStorage.removeItem('netmonitor_user');
  });

export const getToken = () => {
  const t = localStorage.getItem('netmonitor_token');
  return t && t !== 'undefined' ? t : null;
};
