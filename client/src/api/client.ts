import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';
import type {
  ApiResponse, Device, Metric, Alert, AlertCounts,
  DashboardStats, ReportSummary, TimeseriesPoint, DeviceBreakdown, ReportAlert,
  FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint,
  CaptureSession, CapturedPacket, NetworkInterface,
  PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse
} from './types';
export type { Device, Metric, Alert, AlertCounts, DashboardStats, ReportSummary, TimeseriesPoint, DeviceBreakdown, ReportAlert, PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse };

// ── camelCase → snake_case transformer ───────────────────────

function camelToSnake(str: string): string {
  return str.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

function transformKeys(obj: unknown): unknown {
  if (Array.isArray(obj)) return obj.map(transformKeys);
  if (obj !== null && typeof obj === 'object' && !(obj instanceof Date)) {
    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
      result[camelToSnake(key)] = transformKeys(value);
    }
    return result;
  }
  return obj;
}

// ── Go backend response unwrapping ───────────────────────────
// Go wraps responses in { success, data } via httputil.SendOK.

function unwrapGoResponse<T>(raw: unknown): T {
  const transformed = transformKeys(raw) as Record<string, unknown>;
  const data = transformed.data !== undefined ? transformed.data : transformed;
  if (data && typeof data === 'object' && 'alerts' in data && 'total' in data) {
    return (data as Record<string, unknown>).alerts as T;
  }
  return data as T;
}

// Transform raw response data (unwraps Go envelope + converts camelCase → snake_case)
function tx<T>(raw: unknown): T {
  const body = raw as Record<string, unknown>;
  if (body && typeof body === 'object' && 'data' in body) {
    return unwrapGoResponse<T>(body);
  }
  return transformKeys(raw) as T;
}

// Wraps transformed data in { data: ... } to match ApiResponse<T> that pages expect
function wrap<T>(raw: unknown): ApiResponse<T> {
  return { data: tx<T>(raw), success: true } as ApiResponse<T>;
}

// ── Configurable Axios instances ─────────────────────────────

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api',
  timeout: 30_000,
});

const v1 = axios.create({
  baseURL: import.meta.env.VITE_API_V1_URL || '/api/v1',
  timeout: 30_000,
});

// ── Shared auth helpers ──────────────────────────────────────

const attachToken = (config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem('netmonitor_token');
  if (token && token !== 'undefined') {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
};

const clearCredentials = () => {
  localStorage.removeItem('netmonitor_token');
  localStorage.removeItem('netmonitor_refresh_token');
  localStorage.removeItem('netmonitor_user');
};

// ── Token refresh logic ──────────────────────────────────────

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

    // Go returns { success, data: { accessToken, refreshToken } }
    const newToken = (raw as any).data?.accessToken || (raw as any).data?.token;
    localStorage.setItem('netmonitor_token', newToken);
    const newRefresh = (raw as any).data?.refreshToken;
    if (newRefresh) {
      localStorage.setItem('netmonitor_refresh_token', newRefresh);
    }

    processQueue(null, newToken);
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

// ── Attach interceptors ──────────────────────────────────────

api.interceptors.request.use(attachToken);
v1.interceptors.request.use(attachToken);
api.interceptors.response.use((res) => res, handleTokenRefresh);
v1.interceptors.response.use((res) => res, handleTokenRefresh);

// ── Flow Analysis ────────────────────────────────────────────

export const getFlowRecords = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get(`/flows?${qs}`).then((r) => wrap<FlowRecord[]>(r.data));
};

export const getTopTalkers = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get(`/flows/top-talkers?${qs}`).then((r) => wrap<TopTalker[]>(r.data));
};

export const getProtocolDistribution = (params: Record<string, string> = {}) => {
  const qs = new URLSearchParams(params);
  return v1.get(`/flows/protocols?${qs}`).then((r) => wrap<ProtocolBreakdown[]>(r.data));
};

export const getFlowTimeseries = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get(`/flows/timeseries?${qs}`).then((r) => wrap<FlowTimeseriesPoint[]>(r.data));
};

export const getFlowStats = () =>
  v1.get('/flows/stats').then((r) => wrap<FlowStats>(r.data));

// ── Packet Capture ────────────────────────────────────────────

export const getInterfaces = () =>
  v1.get('/capture/interfaces').then((r) => wrap<NetworkInterface[]>(r.data));

export const startCaptureSession = (body: { interface: string; filter?: string }) =>
  v1.post('/capture/start', body).then((r) => wrap<CaptureSession>(r.data));

export const stopCaptureSession = (id: number) =>
  v1.post(`/capture/${id}/stop`).then((r) => wrap<CaptureSession>(r.data));

export const getCaptureSession = (id: number) =>
  v1.get(`/capture/${id}`).then((r) => wrap<CaptureSession>(r.data));

export const getCapturePackets = (sessionId: number, params: Record<string, number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { qs.set(k, String(v)); }
  return v1.get(`/capture/${sessionId}/packets?${qs}`).then((r) => wrap<CapturedPacket[]>(r.data));
};

export const getCaptureSessions = () =>
  v1.get('/capture/sessions').then((r) => wrap<CaptureSession[]>(r.data));

// ── Auth ─────────────────────────────────────────────────────

export const login = (username: string, password: string) =>
  api.post('/auth/login', { username, password }).then((r) => {
    const raw = r.data;
    const token = (raw as any).data?.accessToken || (raw as any).data?.token;
    const refreshToken = (raw as any).data?.refreshToken;
    const user = (raw as any).data?.user;
    localStorage.setItem('netmonitor_token', token);
    localStorage.setItem('netmonitor_refresh_token', refreshToken);
    localStorage.setItem('netmonitor_user', JSON.stringify(user));
    return { success: true, data: { token, refreshToken, user } };
  });

export const logout = () => {
  return api.post('/auth/logout').finally(() => {
    clearCredentials();
  });
};

export const getToken = () => {
  const t = localStorage.getItem('netmonitor_token');
  return t && t !== 'undefined' ? t : null;
};

// ── Devices ──────────────────────────────────────────────────

export const getDevices = () =>
  api.get('/devices').then((r) => wrap<Device[]>(r.data));

export const addDevice = (device: Record<string, unknown>) =>
  api.post('/devices', device).then((r) => wrap<Device>(r.data));

export const deleteDevice = (id: number) =>
  api.delete(`/devices/${id}`).then((r) => r.data);

// ── Metrics ──────────────────────────────────────────────────

export const getLatestMetrics = () =>
  api.get('/metrics/latest').then((r) => wrap<Metric[]>(r.data));

export const getDeviceMetrics = (id: number, limit?: number) =>
  api.get(`/v1/devices/${id}/metrics${limit ? `?limit=${limit}` : ''}`).then((r) => wrap<Metric[]>(r.data));

// ── Ports ────────────────────────────────────────────────────

export const getDevicePorts = (id: number) =>
  api.get(`/devices/${id}/ports`).then((r) => wrap<PortScanResult[]>(r.data));

export const scanDevicePorts = (id: number) =>
  api.post(`/devices/${id}/scan-ports`).then((r) => wrap<PortScanResponse>(r.data));

// ── Alerts ───────────────────────────────────────────────────

export const getAlerts = (status?: string, limit?: number) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  if (limit) qs.set('limit', String(limit));
  return api.get(`/alerts?${qs}`).then((r) => wrap<Alert[]>(r.data));
};

export const getAlertCounts = () =>
  api.get('/alerts/counts').then((r) => wrap<AlertCounts>(r.data));

export const acknowledgeAlert = (id: number) =>
  api.post(`/alerts/${id}/acknowledge`).then((r) => r.data);

export const resolveAlert = (id: number) =>
  api.post(`/alerts/${id}/resolve`).then((r) => r.data);

// ── Dashboard ────────────────────────────────────────────────

export const getStats = () =>
  api.get('/stats').then((r) => wrap<DashboardStats>(r.data));

// ── Insights / AI Health ─────────────────────────────────────

export const getInsights = () =>
  api.get('/insights').then((r) => wrap<InsightsResponse>(r.data));

export const getInsightsHistory = (hours?: number) => {
  const qs = hours ? `?hours=${hours}` : '';
  return api.get(`/insights/history${qs}`).then((r) => wrap<HealthHistoryResponse>(r.data));
};

// ── Reports ──────────────────────────────────────────────────

export const getReportSummary = (query = '') =>
  api.get(`/reports/summary${query}`).then((r) => wrap<ReportSummary>(r.data));

export const getReportTimeseries = (query = '') =>
  api.get(`/reports/timeseries${query}`).then((r) => wrap<TimeseriesPoint[]>(r.data));

export const getReportDeviceBreakdown = (query = '') =>
  api.get(`/reports/devices${query}`).then((r) => wrap<DeviceBreakdown[]>(r.data));

export const getReportAlerts = (query = '') =>
  api.get(`/reports/alerts${query}`).then((r) => wrap<ReportAlert[]>(r.data));

export const downloadMetricsCsv = (query = '') =>
  api.get(`/reports/export${query}`, { responseType: 'blob' }).then((r) => r.data);

export default api;
