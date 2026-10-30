import { v1, wrap, type RequestOptions } from './http';

export interface ISPLink {
  id: number;
  name: string;
  provider: string;
  circuit_id: string;
  bandwidth_mbps: number;
  gateway_ip: string;
  sla_uptime_percent: number;
  cost_monthly: number;
  monitoring_interval_seconds: number;
  enabled: boolean;
}

export interface ISPCompareLink {
  id: number;
  name: string;
  provider: string;
  bandwidthMbps: number;
  gatewayIp: string;
  slaUptime: number;
  avgLatencyMs: number;
  avgJitterMs: number;
  avgPacketLoss: number;
  avgDownload: number;
  avgUpload: number;
  uptimePercent: number;
  totalProbes: number;
  status: string;
}

export interface ISPComparison {
  links: ISPCompareLink[];
  compared_at: string;
}

export interface ISPSLA {
  linkId: number;
  name: string;
  provider: string;
  slaTarget: number | null;
  actualUptime: number;
  slaCompliant: boolean;
  slaGap: number | null;
  avgLatencyMs: number;
  avgJitterMs: number;
  avgPacketLoss: number;
  totalProbes: number;
  latestStatus: string;
}

export interface ISPTimeSeriesPoint {
  timestamp: string;
  latencyMs: number | null;
  jitterMs: number | null;
  packetLoss: number | null;
  downloadMbps: number | null;
  uploadMbps: number | null;
  status: string;
}

export const getISPLinks = ({ signal }: RequestOptions = {}) =>
  v1.get('/isp-links', { signal }).then((r) => wrap<ISPLink[]>(r.data));

export const getISPComparison = ({ signal }: RequestOptions = {}) =>
  v1.get('/isp-links/comparison', { signal }).then((r) => wrap<ISPComparison>(r.data));

export const getISPLinkSLA = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/isp-links/${id}/sla`, { signal }).then((r) => wrap<ISPSLA>(r.data));

export const getISPLinkTimeseries = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/isp-links/${id}/metrics/timeseries`, { signal }).then((r) => wrap<ISPTimeSeriesPoint[]>(r.data));

export const createISPLink = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/isp-links', body, { signal }).then((r) => wrap<ISPLink>(r.data));

export const updateISPLink = (id: number, body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.put(`/isp-links/${id}`, body, { signal }).then((r) => wrap<ISPLink>(r.data));

export const deleteISPLink = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/isp-links/${id}`, { signal }).then((r) => r.data as unknown);
