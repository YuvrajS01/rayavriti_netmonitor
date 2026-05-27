import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';
import type {
  ApiResponse, AuthTokens, Device, Metric, Alert, AlertCounts,
  DashboardStats, ReportSummary, TimeseriesPoint, DeviceBreakdown, ReportAlert,
  FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint,
  CaptureSession, CapturedPacket, NetworkInterface,
  PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse
} from './types';
export type { Device, Metric, Alert, AlertCounts, DashboardStats, ReportSummary, TimeseriesPoint, DeviceBreakdown, ReportAlert, PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse };

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
  if (token) {
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

  // Only attempt refresh on 401, and never retry a retry
  if (error.response?.status !== 401 || originalRequest._retry) {
    return Promise.reject(error);
  }

  // Don't try to refresh the refresh endpoint itself
  if (originalRequest.url?.includes('/auth/refresh')) {
    clearCredentials();
    window.location.href = '/login';
    return Promise.reject(error);
  }

  if (isRefreshing) {
    // Another refresh is in progress — queue this request
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
    if (!refreshToken) throw new Error('No refresh token');

    const { data } = await axios.post<ApiResponse<AuthTokens>>(
      `${import.meta.env.VITE_API_V1_URL || '/api/v1'}/auth/refresh`,
      { refreshToken }
    );

    const newToken = data.data.token;
    localStorage.setItem('netmonitor_token', newToken);
    if (data.data.refreshToken) {
      localStorage.setItem('netmonitor_refresh_token', data.data.refreshToken);
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

// ── Attach interceptors to both clients ──────────────────────

api.interceptors.request.use(attachToken);
v1.interceptors.request.use(attachToken);

api.interceptors.response.use((res) => res, handleTokenRefresh);
v1.interceptors.response.use((res) => res, handleTokenRefresh);

export const getFlowRecords = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get<ApiResponse<FlowRecord[]>>(`/flows?${qs}`).then((r) => r.data);
};

export const getTopTalkers = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get<ApiResponse<TopTalker[]>>(`/flows/top-talkers?${qs}`).then((r) => r.data);
};

export const getProtocolDistribution = (params: Record<string, string> = {}) => {
  const qs = new URLSearchParams(params);
  return v1.get<ApiResponse<ProtocolBreakdown[]>>(`/flows/protocols?${qs}`).then((r) => r.data);
};

export const getFlowTimeseries = (params: Record<string, string | number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return v1.get<ApiResponse<FlowTimeseriesPoint[]>>(`/flows/timeseries?${qs}`).then((r) => r.data);
};

export const getFlowStats = () =>
  v1.get<ApiResponse<FlowStats>>('/flows/stats').then((r) => r.data);

// ── Packet Capture ────────────────────────────────────────────

export const getInterfaces = () =>
  v1.get<ApiResponse<NetworkInterface[]>>('/capture/interfaces').then((r) => r.data);

export const startCaptureSession = (body: { interface: string; filter?: string }) =>
  v1.post<ApiResponse<CaptureSession>>('/capture/start', body).then((r) => r.data);

export const stopCaptureSession = (id: number) =>
  v1.post<ApiResponse<CaptureSession>>(`/capture/${id}/stop`).then((r) => r.data);

export const getCaptureSession = (id: number) =>
  v1.get<ApiResponse<CaptureSession>>(`/capture/${id}`).then((r) => r.data);

export const getCapturePackets = (sessionId: number, params: Record<string, number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { qs.set(k, String(v)); }
  return v1.get<ApiResponse<CapturedPacket[]>>(`/capture/${sessionId}/packets?${qs}`).then((r) => r.data);
};

export const getCaptureSessions = () =>
  v1.get<ApiResponse<CaptureSession[]>>('/capture/sessions').then((r) => r.data);

// ── Auth ─────────────────────────────────────────────────────

export const login = (username: string, password: string) =>
  api.post<ApiResponse<AuthTokens>>('/auth/login', { username, password }).then((r) => {
    const { token, refreshToken, user } = r.data.data;
    localStorage.setItem('netmonitor_token', token);
    localStorage.setItem('netmonitor_refresh_token', refreshToken);
    localStorage.setItem('netmonitor_user', JSON.stringify(user));
    return r.data;
  });

export const logout = () => {
  return api.post('/auth/logout').finally(() => {
    clearCredentials();
  });
};

export const getToken = () => localStorage.getItem('netmonitor_token');

// ── Devices ──────────────────────────────────────────────────

export const getDevices = () =>
  api.get<ApiResponse<Device[]>>('/devices').then((r) => r.data);

export const addDevice = (device: Record<string, unknown>) =>
  api.post<ApiResponse<Device>>('/devices', device).then((r) => r.data);

export const deleteDevice = (id: number) =>
  api.delete<ApiResponse<void>>(`/devices/${id}`).then((r) => r.data);

// ── Metrics ──────────────────────────────────────────────────

export const getLatestMetrics = () =>
  api.get<ApiResponse<Metric[]>>('/metrics/latest').then((r) => r.data);

export const getDeviceMetrics = (id: number, limit?: number) =>
  api.get<ApiResponse<Metric[]>>(`/metrics/device/${id}${limit ? `?limit=${limit}` : ''}`).then((r) => r.data);

// ── Ports ────────────────────────────────────────────────────

export const getDevicePorts = (id: number) =>
  api.get<ApiResponse<PortScanResult[]>>(`/devices/${id}/ports`).then((r) => r.data);

export const scanDevicePorts = (id: number) =>
  api.post<ApiResponse<PortScanResponse>>(`/devices/${id}/scan-ports`).then((r) => r.data);

// ── Alerts ───────────────────────────────────────────────────

export const getAlerts = (status?: string, limit?: number) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  if (limit) qs.set('limit', String(limit));
  return api.get<ApiResponse<Alert[]>>(`/alerts?${qs}`).then((r) => r.data);
};

export const getAlertCounts = () =>
  api.get<ApiResponse<AlertCounts>>('/alerts/counts').then((r) => r.data);

export const acknowledgeAlert = (id: number) =>
  api.post<ApiResponse<void>>(`/alerts/${id}/acknowledge`).then((r) => r.data);

export const resolveAlert = (id: number) =>
  api.post<ApiResponse<void>>(`/alerts/${id}/resolve`).then((r) => r.data);

// ── Dashboard ────────────────────────────────────────────────

export const getStats = () =>
  api.get<ApiResponse<DashboardStats>>('/stats').then((r) => r.data);

// ── Insights / AI Health ─────────────────────────────────────

export const getInsights = () =>
  api.get<ApiResponse<InsightsResponse>>('/insights').then((r) => r.data);

export const getInsightsHistory = (hours?: number) => {
  const qs = hours ? `?hours=${hours}` : '';
  return api.get<ApiResponse<HealthHistoryResponse>>(`/insights/history${qs}`).then((r) => r.data);
};

// ── Reports ──────────────────────────────────────────────────

export const getReportSummary = (query = '') =>
  api.get<ApiResponse<ReportSummary>>(`/reports/summary${query}`).then((r) => r.data);

export const getReportTimeseries = (query = '') =>
  api.get<ApiResponse<TimeseriesPoint[]>>(`/reports/timeseries${query}`).then((r) => r.data);

export const getReportDeviceBreakdown = (query = '') =>
  api.get<ApiResponse<DeviceBreakdown[]>>(`/reports/devices${query}`).then((r) => r.data);

export const getReportAlerts = (query = '') =>
  api.get<ApiResponse<ReportAlert[]>>(`/reports/alerts${query}`).then((r) => r.data);

export const downloadMetricsCsv = (query = '') =>
  api.get(`/reports/metrics.csv${query}`, { responseType: 'blob' }).then((r) => r.data);

export default api;
