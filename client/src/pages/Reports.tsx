import { useState, useCallback, Suspense } from 'react';
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getReportSummary, getReportTimeseries, getReportDeviceBreakdown, getReportAlerts, getReportISP, downloadMetricsCsv, getDevices } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint, DeviceBreakdown, ReportAlert, Device } from '../api/types';
import type { ISPReportLink } from '../api/reports';
import Button from '../components/ui/Button';
import IconButton from '../components/ui/IconButton';
import ErrorState from '../components/ui/ErrorState';
import { TabPanel, Tabs } from '../components/ui/Tabs';
import { useTabParam, useTabsId } from '../hooks/useTabParam';

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
  { id: 'summary', label: 'Executive summary', icon: 'dashboard' },
  { id: 'devices', label: 'Device performance', icon: 'devices' },
  { id: 'sla', label: 'SLA compliance', icon: 'verified' },
  { id: 'alerts', label: 'Alert history', icon: 'notifications' },
  { id: 'isp', label: 'ISP links', icon: 'router' },
];
const TAB_IDS = TABS.map((t) => t.id);

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
  const [activeTab, setActiveTab] = useTabParam(TAB_IDS);
  const tabsId = useTabsId();
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
            <h1 className="text-2xl font-semibold text-fg mb-1">Reports</h1>
            <p className="text-fg-muted max-w-xl text-sm">Historical analytics, SLA compliance and performance across all monitored nodes.</p>
          </div>
          <div className="flex items-center gap-2 no-print">
            <Button variant="secondary" icon="download" onClick={() => { void downloadMetricsCsv(toQuery(from, to, selectedDevice)); }}>
              Export CSV
            </Button>
            <Button variant="secondary" icon="print" onClick={handlePrint}>Print</Button>
          </div>
        </div>
      </header>

      <section aria-label="Report filters" className="bg-surface-1 rounded-lg border border-line p-5 mb-6 no-print">
        <div className="flex flex-col xl:flex-row gap-4 xl:items-end xl:justify-between">
          <div className="flex flex-wrap items-end gap-2">
            <div role="group" aria-label="Quick range" className="inline-flex rounded-md border border-line bg-inset p-0.5">
              {RANGES.map(r => (
                <button
                  key={r.hours}
                  type="button"
                  aria-pressed={activeRange === r.hours}
                  onClick={() => setRange(r.hours)}
                  className={`h-8 min-w-11 rounded-[6px] px-3 text-sm font-medium transition-colors ${activeRange === r.hours ? 'bg-surface-3 text-fg shadow-e1' : 'text-fg-muted hover:text-fg'}`}
                >
                  {r.label}
                </button>
              ))}
            </div>
            <label className="flex flex-col gap-1 text-xs text-fg-muted">
              Device
              <select value={selectedDevice ?? ''} onChange={e => setSelectedDevice(e.target.value ? Number(e.target.value) : undefined)}
                className="h-9 min-w-[160px] rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary">
                <option value="">All devices</option>
                {allDevices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
              </select>
            </label>
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <label className="flex flex-col gap-1 text-xs text-fg-muted">
              From
              <input type="datetime-local" value={from} onChange={e => setFrom(e.target.value)} className="h-9 rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary" />
            </label>
            <label className="flex flex-col gap-1 text-xs text-fg-muted">
              To
              <input type="datetime-local" value={to} onChange={e => setTo(e.target.value)} className="h-9 rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary" />
            </label>
            <Button size="sm" className="h-9" icon={loading ? 'progress_activity' : 'play_arrow'} onClick={refresh} disabled={loading}>
              Run
            </Button>
          </div>
        </div>
      </section>

      {selectedDevice && (
        <div className="flex items-center gap-2 mb-4 no-print">
          <span className="text-xs text-fg-muted">Filtered to</span>
          <span className="inline-flex items-center gap-1 rounded-full border border-line-strong bg-surface-2 py-0.5 pl-3 pr-1 text-xs font-medium text-fg">
            {allDevices.find(d => d.id === selectedDevice)?.name || `Device ${selectedDevice}`}
            <IconButton icon="close" label="Clear device filter" size="sm" className="h-6 w-6" onClick={() => setSelectedDevice(undefined)} />
          </span>
        </div>
      )}

      <Tabs items={TABS} value={activeTab} onChange={setActiveTab} label="Report sections" idBase={tabsId} className="mb-6 no-print" />

      {TABS.map((tab) => (
        <TabPanel key={tab.id} idBase={tabsId} id={tab.id} value={activeTab}>
          {loading ? (
            <ReportLoading message="Loading report data…" />
          ) : error ? (
            <ErrorState message={error} onRetry={refresh} />
          ) : (
            <div className="report-tab-content">
              <Suspense fallback={<ReportLoading message="Loading section…" />}>
                {tab.id === 'summary' && <SummaryTab summary={summary} series={series} />}
                {tab.id === 'devices' && <DeviceTab devices={deviceBreakdown} onSelectDevice={handleSelectDevice} />}
                {tab.id === 'sla' && <SlaTab summary={summary} series={series} />}
                {tab.id === 'alerts' && <AlertTab alerts={reportAlerts} />}
                {tab.id === 'isp' && <IspTab links={ispLinks} />}
              </Suspense>
            </div>
          )}
        </TabPanel>
      ))}

      <div className="print-footer hidden">
        <p>Rayavriti NetMonitor — Report generated {new Date().toLocaleString()}</p>
        <p>Period: {from ? new Date(from).toLocaleString() : '—'} to {to ? new Date(to).toLocaleString() : '—'}</p>
      </div>
    </div>
  );
}

function ReportLoading({ message }: { message: string }) {
  return (
    <div role="status" className="flex flex-col items-center justify-center gap-3 py-16">
      <span className="material-symbols-outlined animate-spin text-3xl text-fg-muted" aria-hidden="true">progress_activity</span>
      <p className="text-sm text-fg-muted">{message}</p>
    </div>
  );
}
