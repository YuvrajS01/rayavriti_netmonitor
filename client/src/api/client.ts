import axios from 'axios';
import type {
  ApiResponse, AuthTokens, Device, Metric, Alert, AlertCounts,
  DashboardStats, ReportSummary, TimeseriesPoint,
  FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint,
  CaptureSession, CapturedPacket, NetworkInterface,
  PortScanResult, PortScanResponse, InsightsResponse
} from './types';

const api = axios.create({ baseURL: '/api' });

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('netmonitor_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('netmonitor_token');
      localStorage.removeItem('netmonitor_user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Auth
export const login = (username: string, password: string) =>
  api.post<ApiResponse<AuthTokens>>('/auth/login', { username, password }).then((r) => r.data);

export const logout = () =>
  api.post('/auth/logout').then(() => {
    localStorage.removeItem('netmonitor_token');
    localStorage.removeItem('netmonitor_user');
  });

export const getMe = () =>
  api.get<ApiResponse<{ user: { id: number; username: string; role: string } }>>('/auth/me').then((r) => r.data);

// Devices
export const getDevices = () =>
  api.get<ApiResponse<Device[]>>('/devices').then((r) => r.data);

export const addDevice = (payload: { name: string; host: string; protocol: string; port: number; interval: number; snmpCommunity?: string; snmpVersion?: string }) =>
  api.post<ApiResponse<Device>>('/devices', payload).then((r) => r.data);

export const deleteDevice = (id: number) =>
  api.delete(`/devices/${id}`);

// Metrics
export const getLatestMetrics = () =>
  api.get<ApiResponse<Metric[]>>('/metrics/latest').then((r) => r.data);

export const getDeviceMetrics = (id: number, limit: number = 100) =>
  api.get<ApiResponse<Metric[]>>(`/metrics/device/${id}?limit=${limit}`).then((r) => r.data);

export const getDevicePorts = (id: number) =>
  api.get<ApiResponse<PortScanResult[]>>(`/devices/${id}/ports`).then((r) => r.data);

export const scanDevicePorts = (id: number, payload: { ports?: number[]; timeoutMs?: number; concurrency?: number } = {}) =>
  api.post<ApiResponse<PortScanResponse>>(`/devices/${id}/scan-ports`, payload).then((r) => r.data);

export const getInsights = () =>
  api.get<ApiResponse<InsightsResponse>>('/insights').then((r) => r.data);

// Alerts
export const getAlerts = (status: string = 'active', limit: number = 200) =>
  api.get<ApiResponse<Alert[]>>(`/alerts?status=${status}&limit=${limit}`).then((r) => r.data);

export const getAlertCounts = () =>
  api.get<ApiResponse<AlertCounts>>('/alerts/counts').then((r) => r.data);

export const acknowledgeAlert = (id: number) =>
  api.post(`/alerts/${id}/acknowledge`).then((r) => r.data);

export const resolveAlert = (id: number) =>
  api.post(`/alerts/${id}/resolve`).then((r) => r.data);

// Stats
export const getStats = () =>
  api.get<ApiResponse<DashboardStats>>('/stats').then((r) => r.data);

// Reports
export const getReportSummary = (query: string = '') =>
  api.get<ApiResponse<ReportSummary>>(`/reports/summary${query}`).then((r) => r.data);

export const getReportTimeseries = (query: string = '') =>
  api.get<ApiResponse<TimeseriesPoint[]>>(`/reports/timeseries${query}`).then((r) => r.data);

export const downloadMetricsCsv = async (query: string = '') => {
  const res = await api.get(`/reports/metrics.csv${query}`, { responseType: 'blob' });
  const url = URL.createObjectURL(res.data);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'metrics-report.csv';
  a.click();
  URL.revokeObjectURL(url);
};

export const getToken = () => localStorage.getItem('netmonitor_token');

// ── Flow Analysis ─────────────────────────────────────────────

const v1 = axios.create({ baseURL: '/api/v1' });
v1.interceptors.request.use((config) => {
  const token = localStorage.getItem('netmonitor_token');
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});
v1.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('netmonitor_token');
      localStorage.removeItem('netmonitor_user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

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

export default api;
