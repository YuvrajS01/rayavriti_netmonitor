import { v1, wrap } from './http';
import type { DashboardStats, SystemInfo } from './types';

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

export const getStats = () =>
  v1.get('/system/stats').then((r) => wrap<DashboardStats>(r.data));
