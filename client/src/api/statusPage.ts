import { v1, wrap, type RequestOptions } from './http';

export interface StatusService {
  id: number;
  name: string;
  description: string;
  group_name: string;
  aggregation: string;
  display_order: number;
  show_response_time: boolean;
  show_uptime: boolean;
  enabled: boolean;
}

export interface StatusIncident {
  id: number;
  title: string;
  message: string;
  severity: string;
  status: string;
  started_at: string;
  resolved_at: string | null;
}

export const getStatusServices = ({ signal }: RequestOptions = {}) =>
  v1.get('/status-page/services', { signal }).then((r) => wrap<StatusService[]>(r.data));

export const getStatusIncidents = ({ signal }: RequestOptions = {}) =>
  v1.get('/status-page/incidents', { signal }).then((r) => wrap<StatusIncident[]>(r.data));

export const createStatusService = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/status-page/services', body, { signal }).then((r) => wrap<StatusService>(r.data));

export const updateStatusService = (id: number, body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.put(`/status-page/services/${id}`, body, { signal }).then((r) => wrap<StatusService>(r.data));

export const deleteStatusService = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/status-page/services/${id}`, { signal }).then((r) => r.data as unknown);
