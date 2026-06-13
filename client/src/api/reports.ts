import { api, wrap } from './http';
import type { ReportSummary, ReportTimeseriesPoint, DeviceBreakdown, ReportAlert } from './types';

export const getReportSummary = (query = '') =>
  api.get(`/reports/summary${query}`).then((r) => wrap<ReportSummary>(r.data));

export const getReportTimeseries = (query = '') =>
  api.get(`/reports/timeseries${query}`).then((r) => wrap<ReportTimeseriesPoint[]>(r.data));

export const getReportDeviceBreakdown = (query = '') =>
  api.get(`/reports/devices${query}`).then((r) => wrap<DeviceBreakdown[]>(r.data));

export const getReportAlerts = (query = '') =>
  api.get(`/reports/alerts${query}`).then((r) => wrap<ReportAlert[]>(r.data));

export const downloadMetricsCsv = async (query = '') => {
  const blob = await api.get(`/reports/export${query}`, { responseType: 'blob' }).then((r) => r.data);
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'metrics.csv';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
};
