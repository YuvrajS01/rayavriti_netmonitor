import { useState, useEffect, useCallback } from 'react';
import { getReportSummary, getReportTimeseries, getReportDeviceBreakdown, getReportAlerts, downloadMetricsCsv, getDevices } from '../api/client';
import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint, DeviceBreakdown, ReportAlert, Device } from '../api/types';
import SummaryTab from '../components/reports/SummaryTab';
import DeviceTab from '../components/reports/DeviceTab';
import SlaTab from '../components/reports/SlaTab';
import AlertTab from '../components/reports/AlertTab';

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

type TabId = 'summary' | 'devices' | 'sla' | 'alerts';
const TABS: { id: TabId; label: string; icon: string }[] = [
  { id: 'summary', label: 'Executive Summary', icon: 'dashboard' },
  { id: 'devices', label: 'Device Performance', icon: 'devices' },
  { id: 'sla', label: 'SLA Compliance', icon: 'verified' },
  { id: 'alerts', label: 'Alert History', icon: 'notifications' },
];

const RANGES = [
  { hours: 1, label: '1h' },
  { hours: 6, label: '6h' },
  { hours: 24, label: '24h' },
  { hours: 168, label: '7d' },
  { hours: 720, label: '30d' },
];

export default function Reports() {
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [series, setSeries] = useState<TimeseriesPoint[]>([]);
  const [deviceBreakdown, setDeviceBreakdown] = useState<DeviceBreakdown[]>([]);
  const [reportAlerts, setReportAlerts] = useState<ReportAlert[]>([]);
  const [allDevices, setAllDevices] = useState<Device[]>([]);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [activeRange, setActiveRange] = useState(24);
  const [activeTab, setActiveTab] = useState<TabId>('summary');
  const [selectedDevice, setSelectedDevice] = useState<number | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setRange = useCallback((hours: number) => {
    const now = new Date();
    const before = new Date(now.getTime() - hours * 60 * 60 * 1000);
    setFrom(formatLocalInput(before));
    setTo(formatLocalInput(now));
    setActiveRange(hours);
  }, []);

  const refresh = useCallback(async () => {
    if (!from || !to) return;
    setLoading(true);
    setError(null);
    const query = toQuery(from, to, selectedDevice);
    try {
      const [sumRes, tsRes, devRes, alertRes] = await Promise.all([
        getReportSummary(query),
        getReportTimeseries(query),
        getReportDeviceBreakdown(query),
        getReportAlerts(query),
      ]);
      setSummary(sumRes.data);
      setSeries(tsRes.data || []);
      setDeviceBreakdown(devRes.data || []);
      setReportAlerts(alertRes.data || []);
    } catch {
      setError('Failed to load report data. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [from, to, selectedDevice]);

  useEffect(() => {
    getDevices().then(r => setAllDevices(r.data || [])).catch(() => {});
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRange(24);
  }, [setRange]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (from && to) refresh();
  }, [from, to, selectedDevice, refresh]);

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
      <header className="mb-8 print-header">
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-2">
          <div>
            <h1 className="font-headline text-4xl font-black text-on-surface uppercase tracking-tight mb-1">Reports</h1>
            <p className="text-on-surface-variant font-body max-w-xl text-sm">Historical analytics, SLA compliance, and performance reports across all monitored nodes.</p>
          </div>
          <div className="flex items-center gap-2 no-print">
            <button onClick={() => downloadMetricsCsv(toQuery(from, to, selectedDevice))} className="px-3 py-2 rounded-lg text-xs border border-primary/40 text-primary font-bold uppercase hover:bg-primary/5 transition-colors flex items-center gap-1.5">
              <span className="material-symbols-outlined text-sm">download</span> CSV
            </button>
            <button onClick={handlePrint} className="px-3 py-2 rounded-lg text-xs border border-outline-variant/30 text-on-surface-variant font-bold uppercase hover:text-primary hover:border-primary/40 transition-colors flex items-center gap-1.5">
              <span className="material-symbols-outlined text-sm">print</span> Print
            </button>
          </div>
        </div>
      </header>

      <div className="bg-surface-container-low rounded-xl border border-outline-variant/20 p-5 mb-6 no-print">
        <div className="flex flex-col xl:flex-row gap-4 xl:items-end xl:justify-between">
          <div className="flex flex-wrap items-center gap-2">
            {RANGES.map(r => (
              <button key={r.hours} onClick={() => setRange(r.hours)}
                className={`px-3 py-2 rounded-lg text-xs border font-bold transition-[border-color,color,background-color] ${activeRange === r.hours ? 'border-primary/40 text-primary bg-primary/5' : 'border-outline-variant/20 text-on-surface-variant hover:text-primary'}`}>
                {r.label}
              </button>
            ))}
            <div className="h-5 w-px bg-outline-variant/30 mx-1" />
            <select value={selectedDevice ?? ''} onChange={e => setSelectedDevice(e.target.value ? Number(e.target.value) : undefined)}
              className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none cursor-pointer min-w-[140px]">
              <option value="">All Devices</option>
              {allDevices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
            </select>
          </div>
          <div className="flex flex-wrap gap-2">
            <input type="datetime-local" value={from} onChange={e => setFrom(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none" />
            <input type="datetime-local" value={to} onChange={e => setTo(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none" />
            <button onClick={refresh} disabled={loading} className="px-4 py-2 rounded-lg text-xs bg-primary text-on-primary font-bold uppercase hover:brightness-110 transition-[filter] flex items-center gap-1.5 disabled:opacity-50">
              {loading ? <span className="material-symbols-outlined text-sm animate-spin">refresh</span> : <span className="material-symbols-outlined text-sm">play_arrow</span>}
              Run
            </button>
          </div>
        </div>
      </div>

      {selectedDevice && (
        <div className="flex items-center gap-2 mb-4 no-print">
          <span className="text-[10px] text-on-surface-variant uppercase tracking-widest">Filtered:</span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-primary/10 border border-primary/30 text-primary text-xs font-bold">
            {allDevices.find(d => d.id === selectedDevice)?.name || `Device ${selectedDevice}`}
            <button onClick={() => setSelectedDevice(undefined)} className="hover:text-on-surface transition-colors">
              <span className="material-symbols-outlined text-sm">close</span>
            </button>
          </span>
        </div>
      )}

      <div className="flex gap-1 mb-6 overflow-x-auto no-print border-b border-outline-variant/20 pb-px">
        {TABS.map(tab => (
          <button key={tab.id} onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-2 px-4 py-3 text-xs font-bold uppercase tracking-widest transition-[color,border-color,background-color] rounded-t-lg whitespace-nowrap ${activeTab === tab.id ? 'text-primary border-b-2 border-primary bg-primary/5' : 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container-highest/50'}`}>
            <span className="material-symbols-outlined text-sm">{tab.icon}</span>
            {tab.label}
          </button>
        ))}
      </div>

      {loading && (
        <div className="flex flex-col items-center justify-center py-16">
          <span className="material-symbols-outlined text-4xl text-primary animate-pulse mb-3">hourglass_top</span>
          <p className="text-sm text-on-surface-variant uppercase tracking-widest">Loading report data...</p>
        </div>
      )}

      {error && !loading && (
        <div className="bg-error/10 border border-error/30 rounded-xl p-6 text-center">
          <span className="material-symbols-outlined text-error text-3xl mb-2">error</span>
          <p className="text-sm text-error font-bold">{error}</p>
          <button onClick={refresh} className="mt-3 text-xs text-on-surface-variant hover:text-primary transition-colors underline">
            Retry
          </button>
        </div>
      )}

      {!loading && !error && (
        <div className="report-tab-content">
          {activeTab === 'summary' && <SummaryTab summary={summary} series={series} />}
          {activeTab === 'devices' && <DeviceTab devices={deviceBreakdown} onSelectDevice={handleSelectDevice} />}
          {activeTab === 'sla' && <SlaTab summary={summary} series={series} />}
          {activeTab === 'alerts' && <AlertTab alerts={reportAlerts} />}
        </div>
      )}

      <div className="print-footer hidden">
        <p>Rayavriti NetMonitor — Report generated {new Date().toLocaleString()}</p>
        <p>Period: {from ? new Date(from).toLocaleString() : '—'} to {to ? new Date(to).toLocaleString() : '—'}</p>
      </div>
    </div>
  );
}
