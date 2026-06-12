import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint } from '../../api/types';
import {
  ComposedChart, Area, Line, XAxis, YAxis, ResponsiveContainer, Tooltip,
  CartesianGrid, Bar, BarChart, Legend,
} from 'recharts';

const TT = { background: 'var(--color-surface-container)', border: '1px solid var(--color-outline-variant)', borderRadius: '8px', fontSize: '12px', color: 'var(--color-on-surface)' };

function KpiCard({ icon, label, value, sub, color = 'text-primary' }: { icon: string; label: string; value: string | number; sub?: string; color?: string }) {
  return (
    <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 flex flex-col gap-2 hover:border-primary/30 transition-all">
      <div className="flex items-center gap-2">
        <span className="material-symbols-outlined text-lg opacity-60">{icon}</span>
        <span className="text-[10px] text-on-surface-variant uppercase tracking-[0.15em] font-bold">{label}</span>
      </div>
      <p className={`font-headline text-3xl font-black ${color}`}>{value}</p>
      {sub && <p className="text-[11px] text-on-surface-variant">{sub}</p>}
    </div>
  );
}

function formatLabel(ts: string, idx: number): string {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? String(idx) : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export default function SummaryTab({ summary, series }: { summary: ReportSummary | null; series: TimeseriesPoint[] }) {
  const chartSeries = series.map((p, i) => ({
    label: formatLabel(p.bucketTime ?? String(i), i),
    availability: Number(p.availabilityPercent ?? 0),
    response: Number(p.avgResponse ?? 0),
    samples: Number(p.sampleCount ?? 0),
    down: Number(p.downCount ?? 0),
  }));

  const downBarData = chartSeries.map(p => ({
    label: p.label,
    Down: p.down,
    Warning: Math.max(0, p.samples - p.down - Math.round((p.availability / 100) * p.samples)),
    Healthy: Math.round((p.availability / 100) * p.samples),
  }));

  const availColor = (summary?.availabilityPercent ?? 0) >= 99 ? 'text-primary' : (summary?.availabilityPercent ?? 0) >= 95 ? 'text-amber-400' : 'text-error';

  return (
    <div className="space-y-6 report-section">
      {/* KPI Cards */}
      {summary && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <KpiCard icon="verified" label="Availability" value={`${summary.availabilityPercent}%`} color={availColor} sub={`${summary.totalSamples - summary.downSamples} of ${summary.totalSamples} checks passed`} />
          <KpiCard icon="speed" label="Avg Response" value={`${summary.averageResponseMs}ms`} color="text-on-surface" sub="Mean response time across all devices" />
          <KpiCard icon="monitoring" label="Total Samples" value={summary.totalSamples.toLocaleString()} color="text-on-surface" sub={`${summary.warningSamples} warnings detected`} />
          <KpiCard icon="cancel" label="Down Events" value={summary.downSamples} color={summary.downSamples > 0 ? 'text-error' : 'text-primary'} sub={summary.downSamples > 0 ? 'Service interruptions detected' : 'No interruptions — all clear'} />
        </div>
      )}

      {/* Combined Chart */}
      <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
        <h3 className="text-sm font-headline font-bold mb-1 uppercase tracking-widest">Availability & Response Time</h3>
        <p className="text-[11px] text-on-surface-variant mb-3">Availability % (left) · Avg response ms (right)</p>
        {chartSeries.length === 0 ? <p className="text-xs text-on-surface-variant text-center py-16">No data for selected range</p> : (
          <ResponsiveContainer width="100%" height={280}>
            <ComposedChart data={chartSeries} margin={{ top: 4, right: 48, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="ag2" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid stroke="#2a2a22" strokeDasharray="4 4" />
              <XAxis dataKey="label" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
              <YAxis yAxisId="left" domain={[0, 100]} tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={v => `${v}%`} width={42} />
              <YAxis yAxisId="right" orientation="right" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={v => `${v}ms`} width={52} />
              <Tooltip contentStyle={TT} formatter={(value: unknown, name: unknown) => { if (name === 'availability') return [`${Number(value ?? 0)}%`, 'Availability']; return [`${Number(value ?? 0)}ms`, 'Avg Response']; }} />
              <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} formatter={v => <span style={{ color: '#c8c5b0' }}>{v === 'availability' ? 'Availability %' : 'Avg Response ms'}</span>} />
              <Area yAxisId="left" type="monotone" dataKey="availability" stroke="#d9fd3a" fill="url(#ag2)" strokeWidth={2} dot={false} />
              <Line yAxisId="right" type="monotone" dataKey="response" stroke="#ff7351" strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
            </ComposedChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Sample Distribution */}
      <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
        <h3 className="text-sm font-headline font-bold mb-1 uppercase tracking-widest">Sample Distribution</h3>
        <p className="text-[11px] text-on-surface-variant mb-3">Count of healthy / down checks per time bucket</p>
        {chartSeries.length === 0 ? <p className="text-xs text-on-surface-variant text-center py-12">No data</p> : (
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={downBarData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }} barSize={16}>
              <XAxis dataKey="label" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
              <Tooltip contentStyle={TT} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
              <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} formatter={v => <span style={{ color: '#c8c5b0' }}>{v}</span>} />
              <Bar dataKey="Healthy" stackId="a" fill="#d9fd3a" />
              <Bar dataKey="Warning" stackId="a" fill="#f59e0b" />
              <Bar dataKey="Down" stackId="a" fill="#ff4444" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
}
