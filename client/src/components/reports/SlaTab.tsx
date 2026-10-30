import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint } from '../../api/types';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip, ReferenceLine, CartesianGrid } from 'recharts';
import { AXIS_TICK_STYLE, GRID_COLOR, SERIES_COLORS, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE } from '../../utils/chartConfig';
import { STATUS_META, type StatusMeta } from '../../utils/status';
import { useChartAnimation } from '../../hooks/useReducedMotion';
import { StatusPill } from '../ui/Status';

const SLA_TARGET = 99.9;
const SERIES = SERIES_COLORS[0];

function slaStatus(pct: number): StatusMeta {
  if (pct >= SLA_TARGET) return { ...STATUS_META.up, label: 'SLA met' };
  if (pct >= 95) return { ...STATUS_META.warning, label: 'SLA breached' };
  return { ...STATUS_META.down, label: 'SLA breached' };
}

function SlaGauge({ value }: { value: number }) {
  const r = 64, cx = 80, cy = 80;
  const circ = 2 * Math.PI * r;
  const pct = Math.min(100, Math.max(0, value));
  const meta = slaStatus(pct);
  const offset = circ - (pct / 100) * circ;
  return (
    <div className="relative grid place-items-center">
      <svg width={160} height={160} className="-rotate-90" role="img" aria-label={`Availability ${pct.toFixed(2)}%, ${meta.label}`}>
        <circle cx={cx} cy={cy} r={r} fill="none" stroke="var(--color-surface-3)" strokeWidth={10} />
        <circle cx={cx} cy={cy} r={r} fill="none" stroke={`var(--color-${meta.token})`} strokeWidth={10}
          strokeLinecap="round" strokeDasharray={circ}
          strokeDashoffset={offset}
          className="gauge-ring"
          style={{ '--gauge-circumference': circ, '--gauge-offset': offset } as React.CSSProperties} />
      </svg>
      <div className="absolute flex flex-col items-center" aria-hidden="true">
        <span className="text-2xl font-semibold tabular-nums text-fg">{pct.toFixed(2)}%</span>
        <span className="text-xs text-fg-muted mt-1">Availability</span>
      </div>
    </div>
  );
}

export default function SlaTab({ summary, series }: { summary: ReportSummary | null; series: TimeseriesPoint[] }) {
  const anim = useChartAnimation();
  const avail = summary?.availabilityPercent ?? 0;
  const status = slaStatus(avail);
  const bucketsMet = series.filter(p => Number(p.availabilityPercent ?? 0) >= SLA_TARGET).length;
  const totalBuckets = series.length;

  const chartData = series.map((p, i) => ({
    label: (() => { const d = new Date(p.bucketTime ?? ''); return isNaN(d.getTime()) ? String(i) : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); })(),
    availability: Number(p.availabilityPercent ?? 0),
  }));

  return (
    <div className="space-y-6 report-section">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <section aria-label="SLA compliance" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col items-center justify-center gap-4">
          <p className="text-sm text-fg-muted">SLA target <span className="font-mono text-fg">{SLA_TARGET}%</span></p>
          <SlaGauge value={avail} />
          <StatusPill status={status} size="md" />
        </section>

        <section aria-labelledby="sla-trend-heading" className="lg:col-span-2 bg-surface-1 rounded-lg p-5 border border-line">
          <dl className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <div><dt className="text-xs text-fg-muted">Target</dt><dd className="text-xl font-semibold text-fg">{SLA_TARGET}%</dd></div>
            <div><dt className="text-xs text-fg-muted">Actual</dt><dd className={`text-xl font-semibold ${status.text}`}>{avail}%</dd></div>
            <div><dt className="text-xs text-fg-muted">Buckets met</dt><dd className="text-xl font-semibold text-fg">{bucketsMet}/{totalBuckets}</dd></div>
            <div><dt className="text-xs text-fg-muted">Down events</dt><dd className="text-xl font-semibold text-fg">{summary?.downSamples ?? 0}</dd></div>
          </dl>
          <h3 id="sla-trend-heading" className="text-sm font-semibold text-fg mb-2">Availability vs SLA target</h3>
          {chartData.length === 0 ? <p className="text-xs text-fg-muted text-center py-12">No data</p> : (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={chartData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="sla-fill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={SERIES} stopOpacity={0.25} />
                    <stop offset="100%" stopColor={SERIES} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="label" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={v => `${v}%`} width={44} />
                <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}%`, 'Availability']} />
                {/* The target is a reference, not an alarm: neutral ink, dashed. */}
                <ReferenceLine y={SLA_TARGET} stroke="var(--color-fg-muted)" strokeDasharray="6 3" strokeWidth={1.5} label={{ value: `SLA ${SLA_TARGET}%`, position: 'insideBottomRight', fill: 'var(--color-fg-muted)', fontSize: 12 }} />
                <Area type="monotone" dataKey="availability" stroke={SERIES} fill="url(#sla-fill)" strokeWidth={2} dot={false} {...anim} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </section>
      </div>
    </div>
  );
}
