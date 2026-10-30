import type { ReportSummary, ReportTimeseriesPoint as TimeseriesPoint } from '../../api/types';
import {
  AreaChart, Area, LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip,
  CartesianGrid, Bar, BarChart, Legend,
} from 'recharts';
import {
  AXIS_TICK_STYLE, GRID_COLOR, LEGEND_STYLE, SERIES_COLORS, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE, legendFormatter,
} from '../../utils/chartConfig';
import { STATUS_META, statusColorVar } from '../../utils/status';
import { useChartAnimation } from '../../hooks/useReducedMotion';
import KpiCard from './KpiCard';
import { availabilityStatus } from './reportStatus';

function formatLabel(ts: string, idx: number): string {
  const d = new Date(ts);
  return isNaN(d.getTime()) ? String(idx) : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

const AVAILABILITY_COLOR = SERIES_COLORS[0];
const RESPONSE_COLOR = SERIES_COLORS[1];

export default function SummaryTab({ summary, series }: { summary: ReportSummary | null; series: TimeseriesPoint[] }) {
  const anim = useChartAnimation();
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
    Up: Math.round((p.availability / 100) * p.samples),
  }));

  return (
    <div className="space-y-6 report-section">
      {summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <KpiCard icon="verified" label="Availability" value={`${summary.availabilityPercent}%`} status={availabilityStatus(summary.availabilityPercent)} sub={`${summary.totalSamples - summary.downSamples} of ${summary.totalSamples} checks passed`} />
          <KpiCard icon="speed" label="Avg response" value={`${summary.averageResponseMs} ms`} sub="Mean response time across all devices" />
          <KpiCard icon="monitoring" label="Total samples" value={summary.totalSamples.toLocaleString()} sub={`${summary.warningSamples} warnings detected`} />
          <KpiCard
            icon="cancel"
            label="Down events"
            value={summary.downSamples}
            status={summary.downSamples > 0 ? { ...STATUS_META.down, label: 'Interruptions' } : { ...STATUS_META.up, label: 'All clear' }}
            sub={summary.downSamples > 0 ? 'Service interruptions detected' : 'No interruptions'}
          />
        </div>
      )}

      {/* One measure per chart: availability and response time have different scales. */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <section className="bg-surface-1 rounded-lg p-5 border border-line" aria-labelledby="summary-availability">
          <h3 id="summary-availability" className="text-sm font-semibold text-fg mb-1">Availability</h3>
          <p className="text-xs text-fg-muted mb-3">Share of passing checks per time bucket</p>
          {chartSeries.length === 0 ? <p className="text-xs text-fg-muted text-center py-16">No data for selected range</p> : (
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={chartSeries} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="summary-availability-fill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={AVAILABILITY_COLOR} stopOpacity={0.25} />
                    <stop offset="100%" stopColor={AVAILABILITY_COLOR} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="label" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={v => `${v}%`} width={44} />
                <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} formatter={(value: unknown) => [`${Number(value ?? 0)}%`, 'Availability']} />
                <Area type="monotone" dataKey="availability" stroke={AVAILABILITY_COLOR} fill="url(#summary-availability-fill)" strokeWidth={2} dot={false} {...anim} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </section>

        <section className="bg-surface-1 rounded-lg p-5 border border-line" aria-labelledby="summary-response">
          <h3 id="summary-response" className="text-sm font-semibold text-fg mb-1">Average response time</h3>
          <p className="text-xs text-fg-muted mb-3">Mean of all devices per time bucket</p>
          {chartSeries.length === 0 ? <p className="text-xs text-fg-muted text-center py-16">No data for selected range</p> : (
            <ResponsiveContainer width="100%" height={240}>
              <LineChart data={chartSeries} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="label" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={v => `${v}ms`} width={52} />
                <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} formatter={(value: unknown) => [`${Number(value ?? 0)} ms`, 'Avg response']} />
                <Line type="monotone" dataKey="response" stroke={RESPONSE_COLOR} strokeWidth={2} dot={false} activeDot={{ r: 4 }} {...anim} />
              </LineChart>
            </ResponsiveContainer>
          )}
        </section>
      </div>

      <section className="bg-surface-1 rounded-lg p-5 border border-line" aria-labelledby="summary-samples">
        <h3 id="summary-samples" className="text-sm font-semibold text-fg mb-1">Sample distribution</h3>
        <p className="text-xs text-fg-muted mb-3">Count of up / warning / down checks per time bucket</p>
        {chartSeries.length === 0 ? <p className="text-xs text-fg-muted text-center py-12">No data</p> : (
          <ResponsiveContainer width="100%" height={220}>
            <BarChart data={downBarData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }} barSize={16}>
              <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="label" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} interval="preserveStartEnd" />
              <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} allowDecimals={false} />
              <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} cursor={{ fill: 'var(--color-surface-2)' }} />
              <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
              {/* These series ARE statuses, so they wear the status colours; the
                  2px surface stroke separates stacked segments. */}
              <Bar dataKey="Up" stackId="a" fill={statusColorVar('up')} stroke="var(--color-surface-1)" strokeWidth={1} {...anim} />
              <Bar dataKey="Warning" stackId="a" fill={statusColorVar('warning')} stroke="var(--color-surface-1)" strokeWidth={1} {...anim} />
              <Bar dataKey="Down" stackId="a" fill={statusColorVar('down')} stroke="var(--color-surface-1)" strokeWidth={1} radius={[4, 4, 0, 0]} {...anim} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </section>
    </div>
  );
}
