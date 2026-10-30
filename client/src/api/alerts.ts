import { v1, wrap, type RequestOptions } from './http';
import type { Alert, AlertBulkAction, AlertBulkResult, AlertCounts, AlertNote, AssignableUser } from './types';

export interface AlertGroup {
  groupId: string;
  ruleId?: number;
  count: number;
  alerts: Alert[];
}

/** The backend caps `limit` on GET /alerts at 200. */
export const ALERT_LIST_LIMIT = 200;

/** The backend accepts at most this many ids per bulk call. */
export const ALERT_BULK_MAX = 500;

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
  v1.post(`/alerts/${id}/acknowledge`, undefined, { signal }).then((r): unknown => r.data);

export const resolveAlert = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/alerts/${id}/resolve`, undefined, { signal }).then((r): unknown => r.data);

/** Acknowledge or resolve many alerts in one call (POST /alerts/bulk). */
export const bulkAlertAction = (ids: number[], action: AlertBulkAction, note?: string, { signal }: RequestOptions = {}) =>
  v1
    .post('/alerts/bulk', note ? { ids, action, note } : { ids, action }, { signal })
    .then((r) => wrap<AlertBulkResult>(r.data));

export const getAlertNotes = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/alerts/${id}/notes`, { signal }).then((r) => wrap<AlertNote[]>(r.data));

export const addAlertNote = (id: number, text: string, { signal }: RequestOptions = {}) =>
  v1.post(`/alerts/${id}/notes`, { text }, { signal }).then((r) => wrap<AlertNote>(r.data));

export const setAlertAssignee = (id: number, userId: number | null, { signal }: RequestOptions = {}) =>
  v1.put(`/alerts/${id}/assignee`, { userId }, { signal }).then((r) => wrap<Alert>(r.data));

export const getAssignableUsers = ({ signal }: RequestOptions = {}) =>
  v1.get('/users/assignable', { signal }).then((r) => wrap<AssignableUser[]>(r.data));
