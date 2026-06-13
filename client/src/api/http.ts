import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api',
  timeout: 30_000,
});

export const v1 = axios.create({
  baseURL: import.meta.env.VITE_API_V1_URL || '/api/v1',
  timeout: 30_000,
});

const attachToken = (config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem('netmonitor_token');
  if (token && token !== 'undefined') {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
};

export const clearCredentials = () => {
  localStorage.removeItem('netmonitor_token');
  localStorage.removeItem('netmonitor_refresh_token');
  localStorage.removeItem('netmonitor_user');
};

let isRefreshing = false;
let failedQueue: Array<{
  resolve: (token: string) => void;
  reject: (err: unknown) => void;
}> = [];

const processQueue = (error: unknown, token: string | null = null) => {
  failedQueue.forEach(({ resolve, reject }) => {
    if (token) resolve(token);
    else reject(error);
  });
  failedQueue = [];
};

const handleTokenRefresh = async (error: AxiosError) => {
  const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };

  if (error.response?.status !== 401 || originalRequest._retry) {
    return Promise.reject(error);
  }

  if (originalRequest.url?.includes('/auth/refresh')) {
    clearCredentials();
    window.location.href = '/login';
    return Promise.reject(error);
  }

  if (isRefreshing) {
    return new Promise<string>((resolve, reject) => {
      failedQueue.push({ resolve, reject });
    }).then((token) => {
      originalRequest.headers.Authorization = `Bearer ${token}`;
      return axios(originalRequest);
    });
  }

  originalRequest._retry = true;
  isRefreshing = true;

  try {
    const refreshToken = localStorage.getItem('netmonitor_refresh_token');
    if (!refreshToken || refreshToken === 'undefined') throw new Error('No refresh token');

    const { data: raw } = await axios.post(
      `${import.meta.env.VITE_API_V1_URL || '/api/v1'}/auth/refresh`,
      { refreshToken }
    );

    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    const newToken = (data as Record<string, unknown>)?.accessToken || (data as Record<string, unknown>)?.token;
    localStorage.setItem('netmonitor_token', newToken as string);
    const newRefresh = (data as Record<string, unknown>)?.refreshToken;
    if (newRefresh) {
      localStorage.setItem('netmonitor_refresh_token', newRefresh as string);
    }

    processQueue(null, newToken as string);
    originalRequest.headers.Authorization = `Bearer ${newToken}`;
    return axios(originalRequest);
  } catch (refreshError) {
    processQueue(refreshError, null);
    clearCredentials();
    window.location.href = '/login';
    return Promise.reject(refreshError);
  } finally {
    isRefreshing = false;
  }
};

api.interceptors.request.use(attachToken);
v1.interceptors.request.use(attachToken);
api.interceptors.response.use((res) => res, handleTokenRefresh);
v1.interceptors.response.use((res) => res, handleTokenRefresh);

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
