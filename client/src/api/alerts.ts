import { v1, wrap } from './http';
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
  return v1.get(`/alerts?${qs}`).then((r) => wrap<Alert[]>(r.data));
};

export const getAlertCounts = () =>
  v1.get('/alerts/counts').then((r) => wrap<AlertCounts>(r.data));

export const getGroupedAlerts = (status?: string) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  return v1.get(`/alerts/grouped?${qs}`).then((r) => wrap<AlertGroup[]>(r.data));
};

export const acknowledgeAlert = (id: number) =>
  v1.post(`/alerts/${id}/acknowledge`).then((r) => r.data);

export const resolveAlert = (id: number) =>
  v1.post(`/alerts/${id}/resolve`).then((r) => r.data);
