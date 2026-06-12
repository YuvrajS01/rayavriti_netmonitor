import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';
import type {
  ApiResponse, Device, Metric, Alert, AlertCounts,
  DashboardStats, ReportSummary, ReportTimeseriesPoint, DeviceBreakdown, ReportAlert,
  FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint,
  CaptureSession, CapturedPacket, NetworkInterface,
  PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse,
  SystemInfo
} from './types';
export type { Device, Metric, Alert, AlertCounts, DashboardStats, ReportSummary, ReportTimeseriesPoint, DeviceBreakdown, ReportAlert, PortScanResult, PortScanResponse, InsightsResponse, HealthHistoryResponse };

// ── Go backend response unwrapping ───────────────────────────
// Go wraps responses in { success, data } via httputil.SendOK.
// Go returns camelCase natively — no key transformation needed.

function unwrapGoResponse<T>(raw: unknown): T {
  const body = raw as Record<string, unknown>;
  const data = body?.data !== undefined ? body.data : body;
  // Special case: alerts list endpoint returns { alerts: [...], total } wrapper
  if (data && typeof data === 'object' && 'alerts' in data && 'total' in data) {
    return (data as Record<string, unknown>).alerts as T;
  }
  return data as T;
}

function wrap<T>(raw: unknown): ApiResponse<T> {
  return { data: unwrapGoResponse<T>(raw), success: true } as ApiResponse<T>;
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
  Promise.all([
    api.get('/insights').then((r) => {
      const raw = r.data;
      const body = raw as Record<string, unknown>;
      return body?.data !== undefined ? body.data : body;
    }),
    api.get('/metrics/latest').then((r) => {
      const raw = r.data;
      const body = raw as Record<string, unknown>;
      return (body?.data !== undefined ? body.data : body) as Array<{ deviceId: number; responseTime: number | null; status: string; timestamp: string }>;
    }).catch(() => [] as Array<{ deviceId: number; responseTime: number | null; status: string; timestamp: string }>),
    api.get('/alerts?status=active').then((r) => {
      const raw = r.data;
      const body = raw as Record<string, unknown>;
      const data = body?.data !== undefined ? body.data : body;
      if (data && typeof data === 'object' && 'alerts' in data && 'total' in data) {
        return (data as Record<string, unknown>).alerts as Array<{ deviceId: number }>;
      }
      return data as Array<{ deviceId: number }>;
    }).catch(() => [] as Array<{ deviceId: number }>),
  ]).then(([inner, metrics, alerts]) => {
    if (!Array.isArray(inner)) {
      return wrap<InsightsResponse>({ data: inner, success: true });
    }

    const items = inner as Array<{ deviceId: number; deviceName: string; score: number; status: string }>;

    const metricByDevice = new Map<number, { responseTime: number; status: string }>();
    for (const m of metrics) {
      const existing = metricByDevice.get(m.deviceId);
      if (!existing || new Date(m.timestamp) > new Date(existing.status)) {
        metricByDevice.set(m.deviceId, { responseTime: m.responseTime ?? 0, status: m.status });
      }
    }

    const alertsByDevice = new Map<number, number>();
    for (const a of alerts) {
      alertsByDevice.set(a.deviceId, (alertsByDevice.get(a.deviceId) || 0) + 1);
    }

    const avgScore = items.length ? Math.round(items.reduce((s, d) => s + d.score, 0) / items.length) : 0;
    const critical = items.filter((d) => d.score < 40).length;
    const watch = items.filter((d) => d.score >= 40 && d.score < 70).length;
    const healthy = items.filter((d) => d.score >= 70).length;

    const health = items.map((d) => {
      const metric = metricByDevice.get(d.deviceId);
      const deviceAlerts = alertsByDevice.get(d.deviceId) || 0;
      const isUp = d.status === 'up' || d.status === 'ok';
      const isDown = d.status === 'down';
      const rt = metric?.responseTime ?? 0;

      const availabilityScore = isUp ? 100 : isDown ? 0 : 50;
      const latencyScore = rt === 0 ? 80 : rt < 200 ? 100 : rt < 500 ? 85 : rt < 1000 ? 65 : rt < 2000 ? 40 : 15;
      const alertScore = deviceAlerts === 0 ? 100 : deviceAlerts === 1 ? 70 : deviceAlerts <= 3 ? 40 : 10;
      const stabilityScore = isUp ? 95 : isDown ? 20 : 50;
      const portScore = 80;

      const issues: Array<{ severity: 'critical' | 'warning' | 'info'; type: string; message: string }> = [];
      if (isDown) issues.push({ severity: 'critical', type: 'availability', message: 'Device is offline' });
      if (rt > 1000) issues.push({ severity: 'warning', type: 'latency', message: `High latency: ${Math.round(rt)}ms` });
      if (deviceAlerts > 0) issues.push({ severity: deviceAlerts > 2 ? 'critical' : 'warning', type: 'alerts', message: `${deviceAlerts} active alert${deviceAlerts > 1 ? 's' : ''}` });
      if (d.status === 'warning' || d.status === 'degraded') issues.push({ severity: 'warning', type: 'status', message: 'Device reporting warnings' });

      return {
        deviceId: d.deviceId,
        deviceName: d.deviceName,
        score: d.score,
        label: (d.score < 40 ? 'critical' : d.score < 60 ? 'risk' : d.score < 80 ? 'watch' : 'healthy') as 'critical' | 'risk' | 'watch' | 'healthy',
        availabilityPercent: availabilityScore,
        avgResponseMs: Math.round(rt),
        activeAlerts: deviceAlerts,
        openPorts: 0,
        samples: 1,
        factors: {
          availability: { score: availabilityScore, weight: 0.3, penalty: 100 - availabilityScore },
          latency: { score: latencyScore, weight: 0.25, penalty: 100 - latencyScore },
          alerts: { score: alertScore, weight: 0.2, penalty: 100 - alertScore },
          stability: { score: stabilityScore, weight: 0.15, penalty: 100 - stabilityScore },
          ports: { score: portScore, weight: 0.1, penalty: 100 - portScore },
        },
        trend: 'stable' as const,
        trendDelta: 0,
        issues,
      };
    });

    return {
      data: {
        generatedAt: new Date().toISOString(),
        networkScore: avgScore,
        healthDistribution: { critical, risk: 0, watch, healthy },
        topRisks: items.filter((d) => d.score < 70).slice(0, 5).map((d) => ({
          deviceId: d.deviceId, deviceName: d.deviceName, score: d.score,
          label: d.score < 40 ? 'critical' : 'risk',
          trend: 'stable' as const, trendDelta: 0, primaryIssue: d.status !== 'up' ? `Status: ${d.status}` : 'No issues',
        })),
        health,
        insights: items.filter((d) => d.score < 70).map((d) => ({
          deviceId: d.deviceId, deviceName: d.deviceName, score: d.score, status: d.status,
          type: 'health', severity: d.score < 40 ? 'critical' as const : 'warning' as const,
          title: `${d.deviceName} — ${d.score}%`, message: d.status !== 'up' ? `Device is ${d.status}` : `Score: ${d.score}%`,
        })),
      },
      success: true,
    };
  });

export const getInsightsHistory = (hours?: number) => {
  const qs = hours ? `?hours=${hours}` : '';
  return api.get(`/insights/history${qs}`).then((r) => wrap<HealthHistoryResponse>(r.data));
};

// ── Reports ──────────────────────────────────────────────────

export const getReportSummary = (query = '') =>
  api.get(`/reports/summary${query}`).then((r) => wrap<ReportSummary>(r.data));

export const getReportTimeseries = (query = '') =>
  api.get(`/reports/timeseries${query}`).then((r) => wrap<ReportTimeseriesPoint[]>(r.data));

export const getReportDeviceBreakdown = (query = '') =>
  api.get(`/reports/devices${query}`).then((r) => wrap<DeviceBreakdown[]>(r.data));

export const getReportAlerts = (query = '') =>
  api.get(`/reports/alerts${query}`).then((r) => wrap<ReportAlert[]>(r.data));

export const downloadMetricsCsv = async (query = '') => {
  const blob = await api.get(`/reports/export${query}`, { responseType: 'blob' }).then((r) => r.data);
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'metrics.csv';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
};

// ── System Info ─────────────────────────────────────────────

const SYSTEM_INFO_FALLBACK: SystemInfo = {
  cpu: { usage: 0, cores: 0, model: 'Unavailable' },
  memory: { used: 0, total: 0, percent: 0 },
  disk: { used: 0, total: 0, percent: 0 },
  uptime: 0,
  loadAvg: [0, 0, 0],
};

export const getSystemInfo = () =>
  v1.get('/system/info').then((r) => {
    const raw = r.data;
    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    return { data: (data as SystemInfo) || SYSTEM_INFO_FALLBACK, success: true };
  }).catch(() => ({ data: SYSTEM_INFO_FALLBACK, success: false }));

export default api;
