import { v1, wrapJson, type RequestOptions } from './http';
import type { Dashboard, DashboardLayout } from './types';

export interface DashboardInput {
  name?: string;
  layout?: DashboardLayout;
  shared?: boolean;
}

/** The caller's dashboards first, then dashboards others share. */
export const getDashboards = ({ signal }: RequestOptions = {}) =>
  v1.get('/dashboards', { signal }).then((r) => wrapJson<Dashboard[] | null>(r.data).data ?? []);

export const getDashboard = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/dashboards/${id}`, { signal }).then((r) => wrapJson<Dashboard>(r.data).data);

export const createDashboard = (input: DashboardInput & { name: string }) =>
  v1.post('/dashboards', input).then((r) => wrapJson<Dashboard>(r.data).data);

/** Omitted fields keep their stored values. */
export const updateDashboard = (id: number, input: DashboardInput) =>
  v1.put(`/dashboards/${id}`, input).then((r) => wrapJson<Dashboard>(r.data).data);

export const deleteDashboard = (id: number) => v1.delete(`/dashboards/${id}`);

/** Pass null to go back to the built-in overview. */
export const setDefaultDashboard = (id: number | null) =>
  id == null ? v1.delete('/dashboards/default') : v1.put(`/dashboards/${id}/default`);
