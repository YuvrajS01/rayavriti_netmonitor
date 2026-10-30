import { v1, wrap, type RequestOptions } from './http';
import type { Alert, AlertCounts } from './types';

export interface AlertGroup {
  groupId: string;
  ruleId?: number;
  count: number;
  alerts: Alert[];
}

export const getAlerts = (status?: string, limit?: number, { signal }: RequestOptions = {}) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  if (limit) qs.set('limit', String(limit));
  return v1.get(`/alerts?${qs}`, { signal }).then((r) => wrap<Alert[]>(r.data));
};

export const getAlertCounts = ({ signal }: RequestOptions = {}) =>
  v1.get('/alerts/counts', { signal }).then((r) => wrap<AlertCounts>(r.data));

export const getGroupedAlerts = (status?: string, { signal }: RequestOptions = {}) => {
  const qs = new URLSearchParams();
  if (status) qs.set('status', status);
  return v1.get(`/alerts/grouped?${qs}`, { signal }).then((r) => wrap<AlertGroup[]>(r.data));
};

export const acknowledgeAlert = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/alerts/${id}/acknowledge`, undefined, { signal }).then((r) => r.data);

export const resolveAlert = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/alerts/${id}/resolve`, undefined, { signal }).then((r) => r.data);
