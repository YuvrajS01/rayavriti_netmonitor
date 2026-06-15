import { api, wrap } from './http';
import type { Alert, AlertCounts } from './types';

export interface AlertGroup {
  groupId: string;
  ruleId?: number;
  count: number;
  alerts: Alert[];
}

export const getAlerts = (status?: string, limit?: number) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  if (limit) qs.set('limit', String(limit));
  return api.get(`/alerts?${qs}`).then((r) => wrap<Alert[]>(r.data));
};

export const getAlertCounts = () =>
  api.get('/alerts/counts').then((r) => wrap<AlertCounts>(r.data));

export const getGroupedAlerts = (status?: string) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  return api.get(`/alerts/grouped?${qs}`).then((r) => wrap<AlertGroup[]>(r.data));
};

export const acknowledgeAlert = (id: number) =>
  api.post(`/alerts/${id}/acknowledge`).then((r) => r.data);

export const resolveAlert = (id: number) =>
  api.post(`/alerts/${id}/resolve`).then((r) => r.data);
