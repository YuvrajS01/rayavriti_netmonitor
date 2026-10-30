import { v1, wrap, type RequestOptions } from './http';
import type { DashboardStats, SystemInfo } from './types';

const SYSTEM_INFO_FALLBACK: SystemInfo = {
  cpu: { usage: 0, cores: 0, model: 'Unavailable' },
  memory: { used: 0, total: 0, percent: 0 },
  disk: { used: 0, total: 0, percent: 0 },
  uptime: 0,
  loadAvg: [0, 0, 0],
};

export const getSystemInfo = ({ signal }: RequestOptions = {}) =>
  v1.get('/system/info', { signal }).then((r) => {
    const raw = r.data;
    const body = raw as Record<string, unknown>;
    const data = body?.data !== undefined ? body.data : body;
    return { data: (data as SystemInfo) || SYSTEM_INFO_FALLBACK, success: true };
  }).catch(() => ({ data: SYSTEM_INFO_FALLBACK, success: false }));

export const getStats = ({ signal }: RequestOptions = {}) =>
  v1.get('/system/stats', { signal }).then((r) => wrap<DashboardStats>(r.data));
