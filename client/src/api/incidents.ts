import { v1, wrap, type RequestOptions } from './http';

export interface Incident {
  id: number;
  title: string;
  description: string;
  severity: string;
  status: string;
  source: string;
  assigned_to: number | null;
  location_id: number | null;
  impact_description: string;
  affected_device_count: number;
  started_at: string;
  acknowledged_at: string | null;
  resolved_at: string | null;
  closed_at: string | null;
  duration_seconds: number | null;
  sla_breached: boolean;
  root_cause: string | null;
  resolution: string | null;
  created_by?: number | null;
}

export interface IncidentStats {
  open: number;
  acknowledged: number;
  resolved: number;
  closed: number;
  sla_breached: number;
  total: number;
  avg_duration_seconds: number | null;
  avg_response_seconds: number | null;
}

export interface IncidentTimelineEntry {
  id: number;
  incidentId: number;
  entryType: string;
  oldValue: string | null;
  newValue: string | null;
  message: string;
  author: string;
  createdAt: string;
}

export interface IncidentDevice {
  id: number;
  name: string;
  ip: string;
  status: string;
}

export type IncidentAction = 'acknowledge' | 'resolve' | 'close';

export const getIncidents = ({ signal }: RequestOptions = {}) =>
  v1.get('/incidents', { signal }).then((r) => wrap<Incident[]>(r.data));

export const getIncidentStats = ({ signal }: RequestOptions = {}) =>
  v1.get('/incidents/stats', { signal }).then((r) => wrap<IncidentStats>(r.data));

export const getIncident = (id: string | number, { signal }: RequestOptions = {}) =>
  v1.get(`/incidents/${id}`, { signal }).then((r) => wrap<Incident>(r.data));

export const getIncidentTimeline = (id: string | number, { signal }: RequestOptions = {}) =>
  v1.get(`/incidents/${id}/timeline`, { signal }).then((r) => wrap<IncidentTimelineEntry[]>(r.data));

export const getIncidentDevices = (id: string | number, { signal }: RequestOptions = {}) =>
  v1.get(`/incidents/${id}/devices`, { signal }).then((r) => wrap<IncidentDevice[]>(r.data));

export const createIncident = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/incidents', body, { signal }).then((r) => wrap<Incident>(r.data));

export const runIncidentAction = (
  id: string | number,
  action: IncidentAction,
  body: Record<string, unknown> = {},
  { signal }: RequestOptions = {},
) => v1.post(`/incidents/${id}/${action}`, body, { signal }).then((r) => r.data as unknown);

export const addIncidentNote = (id: string | number, message: string, { signal }: RequestOptions = {}) =>
  v1.post(`/incidents/${id}/note`, { message }, { signal }).then((r) => r.data as unknown);
