import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint } from '../../api/types';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip, ReferenceLine, CartesianGrid } from 'recharts';

const TT = { background: 'var(--color-surface-container)', border: '1px solid var(--color-outline-variant)', borderRadius: '8px', fontSize: '12px', color: 'var(--color-on-surface)' };
const SLA_TARGET = 99.9;

function SlaGauge({ value }: { value: number }) {
  const r = 64, cx = 80, cy = 80;
  const circ = 2 * Math.PI * r;
  const pct = Math.min(100, Math.max(0, value));
  const color = pct >= SLA_TARGET ? '#d9fd3a' : pct >= 95 ? '#e5a910' : '#ff7351';
  return (
    <div className="relative flex flex-col items-center">
      <svg width={160} height={160} className="transform -rotate-90">
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="#26261d" strokeWidth={10} />
        <circle cx={cx} cy={cy} r={r} fill="none" stroke={color} strokeWidth={10}
          strokeLinecap="round" strokeDasharray={circ}
          strokeDashoffset={circ - (pct / 100) * circ}
          className="gauge-ring"
          style={{ '--gauge-circumference': circ, '--gauge-offset': circ - (pct / 100) * circ } as React.CSSProperties} />
      </svg>
      <div className="absolute flex flex-col items-center" style={{ marginTop: 48 }}>
        <span className="font-headline text-3xl font-bold" style={{ color }}>{pct.toFixed(2)}%</span>
        <span className="text-[10px] text-on-surface-variant uppercase tracking-wide mt-1">Availability</span>
      </div>
    </div>
  );
}

export default function SlaTab({ summary, series }: { summary: ReportSummary | null; series: TimeseriesPoint[] }) {
  const avail = summary?.availabilityPercent ?? 0;
  const met = avail >= SLA_TARGET;
  const bucketsMet = series.filter(p => Number(p.availabilityPercent ?? 0) >= SLA_TARGET).length;
  const totalBuckets = series.length;

  const chartData = series.map((p, i) => ({
    label: (() => { const d = new Date(p.bucketTime ?? ''); return isNaN(d.getTime()) ? String(i) : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); })(),
    availability: Number(p.availabilityPercent ?? 0),
  }));

  return (
    <div className="space-y-6 report-section">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="bg-surface-container-high rounded-lg p-6 border border-outline-variant/20 flex flex-col items-center justify-center relative">
          <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-4">SLA Target: {SLA_TARGET}%</p>
          <SlaGauge value={avail} />
          <div className={`mt-6 inline-flex items-center gap-2 px-4 py-2 rounded-full text-xs font-bold uppercase tracking-wide ${met ? 'bg-primary/10 text-primary border border-primary/30' : 'bg-error/10 text-error border border-error/30'}`}>
            <span className="material-symbols-outlined text-sm">{met ? 'check_circle' : 'cancel'}</span>
            {met ? 'SLA Met' : 'SLA Breached'}
          </div>
        </div>

        <div className="lg:col-span-2 bg-surface-container-high rounded-lg p-6 border border-outline-variant/20">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <div><p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Target</p><p className="font-headline text-xl font-bold text-on-surface">{SLA_TARGET}%</p></div>
            <div><p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Actual</p><p className={`font-headline text-xl font-bold ${avail >= SLA_TARGET ? 'text-primary' : 'text-error'}`}>{avail}%</p></div>
            <div><p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Buckets Met</p><p className="font-headline text-xl font-bold text-on-surface">{bucketsMet}/{totalBuckets}</p></div>
            <div><p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Down Events</p><p className="font-headline text-xl font-bold text-error">{summary?.downSamples ?? 0}</p></div>
          </div>
          <h4 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant mb-2">Availability vs SLA Target</h4>
          {chartData.length === 0 ? <p className="text-xs text-on-surface-variant text-center py-12">No data</p> : (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={chartData} margin={{ top: 4, right: 8, left: -10, bottom: 0 }}>
                <defs>
                  <linearGradient id="slaGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.3} />
                    <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="#2a2a22" strokeDasharray="4 4" />
                <XAxis dataKey="label" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={v => `${v}%`} width={38} />
                <Tooltip contentStyle={TT} formatter={(v: unknown) => [`${Number(v ?? 0)}%`, 'Availability']} />
                <ReferenceLine y={SLA_TARGET} stroke="#ff7351" strokeDasharray="6 3" strokeWidth={2} label={{ value: `SLA ${SLA_TARGET}%`, position: 'right', fill: '#ff7351', fontSize: 10 }} />
                <Area type="monotone" dataKey="availability" stroke="#d9fd3a" fill="url(#slaGrad)" strokeWidth={2} dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  );
}
