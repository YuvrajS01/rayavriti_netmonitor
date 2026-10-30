import { v1, wrap, type RequestOptions } from './http';

export interface LogEvent {
  id: number;
  timestamp: string;
  level: 'trace' | 'debug' | 'info' | 'warn' | 'error' | 'fatal';
  component: string;
  eventType?: string;
  message: string;
  requestId?: string;
  traceId?: string;
  userId?: string;
  actor?: string;
  remoteAddr?: string;
  deviceId?: number;
  protocol?: string;
  method?: string;
  path?: string;
  statusCode?: number;
  durationMs?: number;
  error?: string;
  verboseSessionId?: number;
  attrs?: Record<string, unknown>;
}

export interface LogStats {
  total: number;
  byLevel: Record<string, number>;
  byComponent: Record<string, number>;
  errors: number;
  slowRequests: number;
  slowQueries: number;
}

export interface VerboseSession {
  id: number;
  level: 'debug' | 'trace';
  components: string[];
  deviceIds: number[];
  userIds: string[];
  reason: string;
  startedBy?: number;
  createdAt: string;
  expiresAt: string;
  endedAt?: string;
}

export interface LogFilters {
  level?: string;
  component?: string;
  event_type?: string;
  from?: string;
  to?: string;
  device_id?: string;
  user_id?: string;
  request_id?: string;
  trace_id?: string;
  q?: string;
  limit?: number;
  offset?: number;
}

const params = (filters: LogFilters) =>
  Object.fromEntries(Object.entries(filters).filter(([, v]) => v !== undefined && v !== null && v !== ''));

export const getLogs = (filters: LogFilters, { signal }: RequestOptions = {}) =>
  v1.get('/system/logs', { params: params(filters), signal }).then((r) => ({
    data: wrap<{ events: LogEvent[] }>(r.data).data.events || [],
    total: (r.data?.meta?.total as number) || 0,
  }));

export const getLogStats = (filters: LogFilters, { signal }: RequestOptions = {}) =>
  v1.get('/system/logs/stats', { params: params(filters), signal }).then((r) => wrap<LogStats>(r.data));

export const getVerboseSessions = ({ signal }: RequestOptions = {}) =>
  v1.get('/system/logging/verbose-sessions', { params: { active: true }, signal })
    .then((r) => wrap<{ sessions: VerboseSession[] }>(r.data));

export const createVerboseSession = (body: {
  level: 'debug' | 'trace';
  components: string[];
  deviceIds: number[];
  userIds: string[];
  reason: string;
  durationMinutes: number;
}, { signal }: RequestOptions = {}) => v1.post('/system/logging/verbose-sessions', body, { signal }).then((r) => wrap<VerboseSession>(r.data));

export const stopVerboseSession = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/system/logging/verbose-sessions/${id}/stop`, {}, { signal }).then((r) => wrap<{ stopped: boolean }>(r.data));

export const logsExportUrl = (filters: LogFilters) => {
  const query = new URLSearchParams(params(filters) as Record<string, string>).toString();
  return `/api/v1/system/logs/export${query ? `?${query}` : ''}`;
};
