import { useState, useEffect, useCallback } from 'react';
import {
  ComposedChart, AreaChart, Area, Line, XAxis, YAxis, ResponsiveContainer, Tooltip,
  CartesianGrid, Bar, BarChart, Legend,
} from 'recharts';
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

function formatBucketLabel(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

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

  // Enrich series with formatted labels
  const chartSeries = series.map((p, i) => ({
    label: formatBucketLabel(p.timestamp ?? (p as unknown as { bucket: string }).bucket ?? String(i)),
    availability: Number(p.availabilityPercent ?? 0),
    response: Number(p.avgResponseMs ?? 0),
    samples: Number(p.sampleCount ?? 0),
    down: Number(p.downCount ?? 0),
  }));

  // Down events bar chart
  const downBarData = chartSeries.map((p) => ({
    label: p.label,
    Down: p.down,
    Warning: Math.max(0, p.samples - p.down - Math.round((p.availability / 100) * p.samples)),
    Healthy: Math.round((p.availability / 100) * p.samples),
  }));

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
        <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-7 gap-3 mb-8 text-xs">
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
      <div className="space-y-6">
        {/* Combined dual-axis chart: Availability + Response Time */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-1 uppercase tracking-widest">Availability &amp; Response Time</h3>
          <p className="text-[11px] text-on-surface-variant mb-3">Availability % (left axis) · Avg response ms (right axis)</p>
          {chartSeries.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-16">No data for selected range</p>
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <ComposedChart data={chartSeries} margin={{ top: 4, right: 48, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="availGrad2" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="#2a2a22" strokeDasharray="4 4" />
                <XAxis
                  dataKey="label"
                  tick={{ fill: '#8a8a78', fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  interval="preserveStartEnd"
                />
                {/* Left Y: Availability */}
                <YAxis
                  yAxisId="left"
                  domain={[0, 100]}
                  tick={{ fill: '#8a8a78', fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => `${v}%`}
                  width={42}
                />
                {/* Right Y: Response */}
                <YAxis
                  yAxisId="right"
                  orientation="right"
                  tick={{ fill: '#8a8a78', fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => `${v}ms`}
                  width={52}
                />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: unknown, name: unknown) => {
                    if (name === 'availability') return [`${Number(value ?? 0)}%`, 'Availability'];
                    return [`${Number(value ?? 0)}ms`, 'Avg Response'];
                  }}
                />
                <Legend
                  wrapperStyle={{ fontSize: 11, paddingTop: 8 }}
                  formatter={(v) => <span style={{ color: '#c8c5b0' }}>{v === 'availability' ? 'Availability %' : 'Avg Response ms'}</span>}
                />
                <Area
                  yAxisId="left"
                  type="monotone"
                  dataKey="availability"
                  stroke="#d9fd3a"
                  fill="url(#availGrad2)"
                  strokeWidth={2}
                  dot={false}
                />
                <Line
                  yAxisId="right"
                  type="monotone"
                  dataKey="response"
                  stroke="#ff7351"
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                />
              </ComposedChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Two column: individual area charts */}
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          {/* Availability Trend */}
          <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
            <h3 className="text-sm font-headline font-bold mb-2 uppercase tracking-widest">Availability Trend</h3>
            {chartSeries.length === 0 ? (
              <p className="text-xs text-on-surface-variant text-center py-12">No data</p>
            ) : (
              <ResponsiveContainer width="100%" height={220}>
                <AreaChart data={chartSeries} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                  <defs>
                    <linearGradient id="availGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.35} />
                      <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="label" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                  <YAxis domain={[0, 100]} tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}%`} width={38} />
                  <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}%`, 'Availability']} />
                  <Area type="monotone" dataKey="availability" stroke="#d9fd3a" fill="url(#availGrad)" strokeWidth={2} dot={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* Response Trend */}
          <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
            <h3 className="text-sm font-headline font-bold mb-2 uppercase tracking-widest">Response Time Trend</h3>
            {chartSeries.length === 0 ? (
              <p className="text-xs text-on-surface-variant text-center py-12">No data</p>
            ) : (
              <ResponsiveContainer width="100%" height={220}>
                <AreaChart data={chartSeries} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                  <defs>
                    <linearGradient id="respGrad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="#ff7351" stopOpacity={0.35} />
                      <stop offset="100%" stopColor="#ff7351" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <XAxis dataKey="label" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                  <YAxis tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} width={48} />
                  <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}ms`, 'Avg Response']} />
                  <Area type="monotone" dataKey="response" stroke="#ff7351" fill="url(#respGrad)" strokeWidth={2} dot={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>

        {/* Sample count / down events bar chart */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-1 uppercase tracking-widest">Sample Distribution</h3>
          <p className="text-[11px] text-on-surface-variant mb-3">Count of healthy / down checks per time bucket</p>
          {chartSeries.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-12">No data</p>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={downBarData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }} barSize={16}>
                <XAxis dataKey="label" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} formatter={(v) => <span style={{ color: '#c8c5b0' }}>{v}</span>} />
                <Bar dataKey="Healthy" stackId="a" fill="#d9fd3a" />
                <Bar dataKey="Warning" stackId="a" fill="#f59e0b" />
                <Bar dataKey="Down" stackId="a" fill="#ff4444" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  );
}
