import { useState, useCallback, Suspense } from 'react';
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getReportSummary, getReportTimeseries, getReportDeviceBreakdown, getReportAlerts, getReportISP, downloadMetricsCsv, getDevices } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint, DeviceBreakdown, ReportAlert, Device } from '../api/types';
import type { ISPReportLink } from '../api/reports';

const EMPTY_SERIES: TimeseriesPoint[] = [];
const EMPTY_BREAKDOWN: DeviceBreakdown[] = [];
const EMPTY_REPORT_ALERTS: ReportAlert[] = [];
const EMPTY_ISP: ISPReportLink[] = [];
const EMPTY_DEVICES: Device[] = [];

const SummaryTab = lazyWithRetry(() => import('../components/reports/SummaryTab'));
const DeviceTab = lazyWithRetry(() => import('../components/reports/DeviceTab'));
const SlaTab = lazyWithRetry(() => import('../components/reports/SlaTab'));
const AlertTab = lazyWithRetry(() => import('../components/reports/AlertTab'));
const IspTab = lazyWithRetry(() => import('../components/reports/IspTab'));

function formatLocalInput(date: Date) {
  const d = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return d.toISOString().slice(0, 16);
}

function toQuery(from: string, to: string, deviceId?: number) {
  const params = new URLSearchParams();
  if (from) params.set('from', new Date(from).toISOString());
  if (to) params.set('to', new Date(to).toISOString());
  if (deviceId) params.set('deviceId', String(deviceId));
  const text = params.toString();
  return text ? `?${text}` : '';
}

type TabId = 'summary' | 'devices' | 'sla' | 'alerts' | 'isp';
const TABS: { id: TabId; label: string; icon: string }[] = [
  { id: 'summary', label: 'Executive Summary', icon: 'dashboard' },
  { id: 'devices', label: 'Device Performance', icon: 'devices' },
  { id: 'sla', label: 'SLA Compliance', icon: 'verified' },
  { id: 'alerts', label: 'Alert History', icon: 'notifications' },
  { id: 'isp', label: 'ISP Links', icon: 'router' },
];

const RANGES = [
  { hours: 1, label: '1h' },
  { hours: 6, label: '6h' },
  { hours: 24, label: '24h' },
  { hours: 168, label: '7d' },
  { hours: 720, label: '30d' },
];

function rangeInputs(hours: number) {
  const now = new Date();
  const before = new Date(now.getTime() - hours * 60 * 60 * 1000);
  return { from: formatLocalInput(before), to: formatLocalInput(now) };
}

const DEFAULT_RANGE_HOURS = 24;

export default function Reports() {
  const queryClient = useQueryClient();
  const [range, setRangeInputs] = useState(() => rangeInputs(DEFAULT_RANGE_HOURS));
  const { from, to } = range;
  const [activeRange, setActiveRange] = useState(DEFAULT_RANGE_HOURS);
  const [activeTab, setActiveTab] = useState<TabId>('summary');
  const [selectedDevice, setSelectedDevice] = useState<number | undefined>(undefined);

  const setRange = useCallback((hours: number) => {
    setRangeInputs(rangeInputs(hours));
    setActiveRange(hours);
  }, []);
  const setFrom = (value: string) => setRangeInputs((r) => ({ ...r, from: value }));
  const setTo = (value: string) => setRangeInputs((r) => ({ ...r, to: value }));

  // Typing in the datetime inputs shouldn't fire five requests per keystroke.
  const debouncedRange = useDebouncedValue(range, 400);
  const query = toQuery(debouncedRange.from, debouncedRange.to, selectedDevice);
  const enabled = Boolean(debouncedRange.from && debouncedRange.to);

  // Each query is keyed by the filter, so a slow response for an old filter
  // can never overwrite the current one (the race the old effect had).
  const summaryQuery = useQuery({ queryKey: queryKeys.reports.summary(query), queryFn: ({ signal }) => getReportSummary(query, { signal }), enabled });
  const seriesQuery = useQuery({ queryKey: queryKeys.reports.timeseries(query), queryFn: ({ signal }) => getReportTimeseries(query, { signal }), enabled });
  const breakdownQuery = useQuery({ queryKey: queryKeys.reports.devices(query), queryFn: ({ signal }) => getReportDeviceBreakdown(query, { signal }), enabled });
  const alertsQuery = useQuery({ queryKey: queryKeys.reports.alerts(query), queryFn: ({ signal }) => getReportAlerts(query, { signal }), enabled });
  const ispQuery = useQuery({ queryKey: queryKeys.reports.isp(query), queryFn: ({ signal }) => getReportISP(query, { signal }), enabled });
  const devicesQuery = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }) });

  const reportQueries = [summaryQuery, seriesQuery, breakdownQuery, alertsQuery, ispQuery];
  const loading = reportQueries.some((q) => q.isFetching);
  const error = reportQueries.some((q) => q.isError) ? 'Failed to load report data. Please try again.' : null;
  const refresh = () => { void queryClient.invalidateQueries({ queryKey: queryKeys.reports.all }); };

  const summary: ReportSummary | null = summaryQuery.data?.data ?? null;
  const series = seriesQuery.data?.data || EMPTY_SERIES;
  const deviceBreakdown = breakdownQuery.data?.data || EMPTY_BREAKDOWN;
  const reportAlerts = alertsQuery.data?.data || EMPTY_REPORT_ALERTS;
  const ispLinks = ispQuery.data?.data || EMPTY_ISP;
  const allDevices = devicesQuery.data?.data || EMPTY_DEVICES;

  const handleSelectDevice = (id: number) => {
    setSelectedDevice(prev => prev === id ? undefined : id);
  };

  const handlePrint = () => {
    document.body.classList.add('print-report');
    window.print();
    document.body.classList.remove('print-report');
  };

  return (
    <div>
      <header className="mb-6 print-header">
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-2">
          <div>
            <h1 className="font-sans text-2xl font-semibold text-fg mb-1">Reports</h1>
            <p className="text-fg-muted font-sans max-w-xl text-sm">Historical analytics, SLA compliance, and performance reports across all monitored nodes.</p>
          </div>
          <div className="flex items-center gap-2 no-print">
            <button onClick={() => { void downloadMetricsCsv(toQuery(from, to, selectedDevice)); }} className="px-4 py-2 rounded-lg text-sm border border-primary/40 text-primary font-semibold hover:bg-primary/5 transition-colors flex items-center gap-1.5">
              <span className="material-symbols-outlined text-sm">download</span> CSV
            </button>
            <button onClick={handlePrint} className="px-4 py-2 rounded-lg text-sm border border-line text-fg-muted font-semibold hover:text-fg hover:border-line-strong/40 transition-colors flex items-center gap-1.5">
              <span className="material-symbols-outlined text-sm">print</span> Print
            </button>
          </div>
        </div>
      </header>

      <div className="bg-surface-1 rounded-lg border border-line p-5 mb-6 no-print">
        <div className="flex flex-col xl:flex-row gap-4 xl:items-end xl:justify-between">
          <div className="flex flex-wrap items-center gap-2">
            {RANGES.map(r => (
              <button key={r.hours} onClick={() => setRange(r.hours)}
                className={`px-3 py-2 rounded-lg text-xs border font-semibold transition-[border-color,color,background-color] ${activeRange === r.hours ? 'border-primary/40 text-primary bg-primary/5' : 'border-line text-fg-muted hover:text-fg'}`}>
                {r.label}
              </button>
            ))}
            <div className="h-5 w-px bg-line/30 mx-1" />
            <select value={selectedDevice ?? ''} onChange={e => setSelectedDevice(e.target.value ? Number(e.target.value) : undefined)}
              className="bg-inset border border-line rounded-lg px-3 py-2 text-xs text-fg outline-none cursor-pointer min-w-[140px]">
              <option value="">All Devices</option>
              {allDevices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
            </select>
          </div>
          <div className="flex flex-wrap gap-2">
            <input type="datetime-local" value={from} onChange={e => setFrom(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2 text-xs text-fg outline-none" />
            <input type="datetime-local" value={to} onChange={e => setTo(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2 text-xs text-fg outline-none" />
            <button onClick={refresh} disabled={loading} className="px-4 py-2 rounded-lg text-sm bg-primary text-on-primary font-semibold hover:bg-primary/90 transition-colors flex items-center gap-1.5 disabled:opacity-50">
              {loading ? <span className="material-symbols-outlined text-sm animate-spin">refresh</span> : <span className="material-symbols-outlined text-sm">play_arrow</span>}
              Run
            </button>
          </div>
        </div>
      </div>

      {selectedDevice && (
        <div className="flex items-center gap-2 mb-4 no-print">
          <span className="text-xs text-fg-muted">Filtered:</span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-primary/10 border border-primary/30 text-primary text-xs font-semibold">
            {allDevices.find(d => d.id === selectedDevice)?.name || `Device ${selectedDevice}`}
            <button onClick={() => setSelectedDevice(undefined)} className="hover:text-fg transition-colors">
              <span className="material-symbols-outlined text-sm">close</span>
            </button>
          </span>
        </div>
      )}

      <div className="flex gap-1 mb-6 overflow-x-auto no-print border-b border-line pb-px">
        {TABS.map(tab => (
          <button key={tab.id} onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-2 px-4 py-3 text-xs font-semibold transition-[color,border-color,background-color] rounded-t-lg whitespace-nowrap ${activeTab === tab.id ? 'text-primary border-b-2 border-primary bg-primary/5': 'text-fg-muted hover:text-fg hover:bg-inset/50'}`}>
            <span className="material-symbols-outlined text-sm">{tab.icon}</span>
            {tab.label}
          </button>
        ))}
      </div>

      {loading && (
        <div className="flex flex-col items-center justify-center py-16">
          <span className="material-symbols-outlined text-4xl text-primary mb-3">hourglass_top</span>
          <p className="text-sm text-fg-muted">Loading report data...</p>
        </div>
      )}

      {error && !loading && (
        <div className="bg-error/10 border border-error/30 rounded-lg p-6 text-center">
          <span className="material-symbols-outlined text-error text-3xl mb-2">error</span>
          <p className="text-sm text-error font-semibold">{error}</p>
          <button onClick={refresh} className="mt-3 text-xs text-fg-muted hover:text-fg transition-colors underline">
            Retry
          </button>
        </div>
      )}

      {!loading && !error && (
        <div className="report-tab-content">
          <Suspense fallback={
            <div className="flex flex-col items-center justify-center py-16">
              <span className="material-symbols-outlined text-4xl text-primary mb-3">hourglass_top</span>
              <p className="text-sm text-fg-muted">Loading tab...</p>
            </div>
          }>
            {activeTab === 'summary' && <SummaryTab summary={summary} series={series} />}
            {activeTab === 'devices' && <DeviceTab devices={deviceBreakdown} onSelectDevice={handleSelectDevice} />}
            {activeTab === 'sla' && <SlaTab summary={summary} series={series} />}
            {activeTab === 'alerts' && <AlertTab alerts={reportAlerts} />}
            {activeTab === 'isp' && <IspTab links={ispLinks} />}
          </Suspense>
        </div>
      )}

      <div className="print-footer hidden">
        <p>Rayavriti NetMonitor — Report generated {new Date().toLocaleString()}</p>
        <p>Period: {from ? new Date(from).toLocaleString() : '—'} to {to ? new Date(to).toLocaleString() : '—'}</p>
      </div>
    </div>
  );
}
