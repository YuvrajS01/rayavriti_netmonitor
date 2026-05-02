import { useState, useEffect, useCallback } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { getReportSummary, getReportTimeseries, downloadMetricsCsv } from '../api/client';
import type { ReportSummary, TimeseriesPoint } from '../api/types';

function formatLocalInput(date: Date) {
  const d = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return d.toISOString().slice(0, 16);
}

function toQuery(from: string, to: string) {
  const params = new URLSearchParams();
  if (from) params.set('from', new Date(from).toISOString());
  if (to) params.set('to', new Date(to).toISOString());
  const text = params.toString();
  return text ? `?${text}` : '';
}

export default function Reports() {
  const [summary, setSummary] = useState<ReportSummary | null>(null);
  const [series, setSeries] = useState<TimeseriesPoint[]>([]);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [activeRange, setActiveRange] = useState(24);

  const setRange = useCallback((hours: number) => {
    const now = new Date();
    const before = new Date(now.getTime() - hours * 60 * 60 * 1000);
    setFrom(formatLocalInput(before));
    setTo(formatLocalInput(now));
    setActiveRange(hours);
  }, []);

  const refresh = useCallback(async () => {
    const query = toQuery(from, to);
    const [sumRes, tsRes] = await Promise.all([
      getReportSummary(query),
      getReportTimeseries(query),
    ]);
    setSummary(sumRes.data);
    setSeries(tsRes.data || []);
  }, [from, to]);

  useEffect(() => { setRange(24); }, [setRange]);
  useEffect(() => { if (from && to) refresh(); }, [from, to, refresh]);

  const availData = series.map((p, i) => ({ name: i.toString(), value: Number(p.availabilityPercent || 0) }));
  const responseData = series.map((p, i) => ({ name: i.toString(), value: Number(p.avgResponseMs || 0) }));

  const ranges = [
    { hours: 24, label: 'Last 24h' },
    { hours: 168, label: '7 days' },
    { hours: 720, label: '30 days' },
  ];

  return (
    <div>
      <header className="mb-12">
        <h1 className="font-headline text-4xl font-black text-on-surface uppercase tracking-tight mb-2">Reports</h1>
        <p className="text-on-surface-variant font-body max-w-xl">Historical analytics and performance reports across all monitored nodes.</p>
      </header>

      {/* Controls */}
      <div className="bg-surface-container-low rounded-xl border border-outline-variant/20 p-6 mb-8">
        <div className="flex flex-col xl:flex-row gap-4 xl:items-end xl:justify-between">
          <div className="flex flex-wrap gap-2">
            {ranges.map((r) => (
              <button
                key={r.hours}
                onClick={() => setRange(r.hours)}
                className={`px-3 py-2 rounded-lg text-xs border font-bold transition-all ${
                  activeRange === r.hours
                    ? 'border-primary/40 text-primary bg-primary/5'
                    : 'border-outline-variant/20 text-on-surface-variant hover:text-primary'
                }`}
              >
                {r.label}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap gap-2">
            <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none" />
            <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none" />
            <button onClick={refresh} className="px-3 py-2 rounded-lg text-xs bg-primary text-on-primary font-bold uppercase">Run</button>
            <button onClick={() => downloadMetricsCsv(toQuery(from, to))} className="px-3 py-2 rounded-lg text-xs border border-primary/40 text-primary font-bold uppercase">CSV</button>
          </div>
        </div>
      </div>

      {/* Summary Cards */}
      {summary && (
        <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-7 gap-3 mb-6 text-xs">
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">From</p><p className="font-bold truncate">{summary.from}</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">To</p><p className="font-bold truncate">{summary.to}</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">Samples</p><p className="font-bold">{summary.totalSamples}</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">Down</p><p className="font-bold text-error">{summary.downSamples}</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">Warn</p><p className="font-bold text-amber-400">{summary.warningSamples}</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">Availability</p><p className="font-bold text-primary">{summary.availabilityPercent}%</p></div>
          <div className="bg-surface-container-high p-3 rounded-lg"><p className="text-on-surface-variant">Avg Response</p><p className="font-bold">{summary.averageResponseMs} ms</p></div>
        </div>
      )}

      {/* Charts */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-2 uppercase">Availability Trend</h3>
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={availData}>
              <defs>
                <linearGradient id="availGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="name" hide />
              <YAxis domain={[0, 100]} hide />
              <Tooltip contentStyle={{ background: '#1a1a13', border: '1px solid #494840', borderRadius: '8px', fontSize: '12px', color: '#f4f1e6' }} formatter={(v: number) => [`${v}%`, 'Availability']} />
              <Area type="monotone" dataKey="value" stroke="#d9fd3a" fill="url(#availGrad)" strokeWidth={3} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-2 uppercase">Response Trend</h3>
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={responseData}>
              <defs>
                <linearGradient id="respGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#ff7351" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="#ff7351" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="name" hide />
              <YAxis hide />
              <Tooltip contentStyle={{ background: '#1a1a13', border: '1px solid #494840', borderRadius: '8px', fontSize: '12px', color: '#f4f1e6' }} formatter={(v: number) => [`${v}ms`, 'Response']} />
              <Area type="monotone" dataKey="value" stroke="#ff7351" fill="url(#respGrad)" strokeWidth={3} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
