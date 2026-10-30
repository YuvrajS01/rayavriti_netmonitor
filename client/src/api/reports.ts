import { v1, wrap, saveBlob, type RequestOptions } from './http';
import type { ReportSummary, ReportTimeseriesPoint, DeviceBreakdown, ReportAlert } from './types';

export const getReportSummary = (query = '', { signal }: RequestOptions = {}) =>
  v1.get(`/reports/summary${query}`, { signal }).then((r) => wrap<ReportSummary>(r.data));

export const getReportTimeseries = (query = '', { signal }: RequestOptions = {}) =>
  v1.get(`/reports/timeseries${query}`, { signal }).then((r) => wrap<ReportTimeseriesPoint[]>(r.data));

export const getReportDeviceBreakdown = (query = '', { signal }: RequestOptions = {}) =>
  v1.get(`/reports/devices${query}`, { signal }).then((r) => wrap<DeviceBreakdown[]>(r.data));

export const getReportAlerts = (query = '', { signal }: RequestOptions = {}) =>
  v1.get(`/reports/alerts${query}`, { signal }).then((r) => wrap<ReportAlert[]>(r.data));

export interface ISPReportLink {
  id: number;
  name: string;
  provider: string;
  bandwidthMbps: number;
  slaTarget: number;
  avgLatency: number;
  avgJitter: number;
  avgPacketLoss: number;
  avgDownload: number;
  avgUpload: number;
  totalProbes: number;
  uptimePercent: number;
}

export const getReportISP = (query = '', { signal }: RequestOptions = {}) =>
  v1.get(`/reports/isp-links${query}`, { signal }).then((r) => wrap<ISPReportLink[]>(r.data));

export const downloadMetricsCsv = async (query = '', { signal }: RequestOptions = {}) => {
  const blob = await v1.get(`/reports/export${query}`, { responseType: 'blob', signal }).then((r) => r.data as Blob);
  saveBlob(blob, 'metrics.csv');
};

// ── Scheduled / generated reports (Report Builder) ─────────────────────────

export interface ScheduledReport {
  id: number;
  name: string;
  report_type: string;
  format: string;
  schedule_cron: string;
  timezone: string;
  scope_type: string;
  recipients: string[];
  lookback_period: string;
  last_run_at: string | null;
  last_run_status: string | null;
  enabled: boolean;
}

export interface GeneratedReport {
  id: number;
  scheduled_report_id: number;
  report_type: string;
  title: string;
  format: string;
  file_path: string;
  file_size_bytes: number;
  period_from: string;
  period_to: string;
  generated_by: string;
}

export const getScheduledReports = ({ signal }: RequestOptions = {}) =>
  v1.get('/reports/scheduled', { signal }).then((r) => wrap<ScheduledReport[]>(r.data));

export const getGeneratedReports = ({ signal }: RequestOptions = {}) =>
  v1.get('/reports/generated', { signal }).then((r) => wrap<GeneratedReport[]>(r.data));

export const createScheduledReport = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/reports/scheduled', body, { signal }).then((r) => wrap<ScheduledReport>(r.data));

export const runScheduledReport = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/reports/scheduled/${id}/run`, undefined, { signal }).then((r): unknown => r.data);

export const deleteScheduledReport = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/reports/scheduled/${id}`, { signal }).then((r): unknown => r.data);

export const downloadGeneratedReport = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/reports/generated/${id}/download`, { responseType: 'blob', signal }).then((r) => r.data as Blob);
