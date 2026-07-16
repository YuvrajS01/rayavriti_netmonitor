import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';

export const v1 = axios.create({
  baseURL: import.meta.env.VITE_API_V1_URL || '/api/v1',
  timeout: 30_000,
  withCredentials: true,
});

let accessToken: string | null = null;

export const setAccessToken = (token: string | null) => {
  accessToken = token && token !== 'undefined' ? token : null;
};

export const getAccessToken = () => accessToken;

const attachToken = (config: InternalAxiosRequestConfig) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`;
  }
  return config;
};

export const clearCredentials = () => {
  setAccessToken(null);
  localStorage.removeItem('netmonitor_user');
  // Clear cookies via logout endpoint (best-effort)
  axios.post(
    `${import.meta.env.VITE_API_V1_URL || '/api/v1'}/auth/logout`,
    {},
    { withCredentials: true, timeout: 5_000 }
  ).catch(() => {});
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
	const responseBody = error.response?.data as { error?: { message?: string } } | { error?: string } | undefined;
	const message = typeof responseBody?.error === 'string' ? responseBody.error : responseBody?.error?.message;
	if (error.response?.status === 503 && message === 'System is under maintenance') {
		window.location.href = '/system-maintenance';
		return Promise.reject(error);
	}

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
    const { data: raw } = await axios.post(
      `${import.meta.env.VITE_API_V1_URL || '/api/v1'}/auth/refresh`,
      {},
      { timeout: 10_000, withCredentials: true }
    );

    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    const newToken = (data as Record<string, unknown>)?.accessToken || (data as Record<string, unknown>)?.token;
    setAccessToken(newToken as string);

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

v1.interceptors.request.use(attachToken);
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
