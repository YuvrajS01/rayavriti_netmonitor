import type { ReactNode } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend, CartesianGrid } from 'recharts';
import { useQuery } from '@tanstack/react-query';
import { getISPLinkSLA, getISPLinkTimeseries, type ISPLink } from '../api/isp';
import { queryKeys } from '../api/queryKeys';
import {
  AXIS_TICK_STYLE,
  GRID_COLOR,
  LEGEND_STYLE,
  SERIES_COLORS,
  TOOLTIP_ITEM_STYLE,
  TOOLTIP_LABEL_STYLE,
  TOOLTIP_STYLE,
  legendFormatter,
} from '../utils/chartConfig';
import { formatMbps } from '../utils/formatters';
import { useChartAnimation } from '../hooks/useReducedMotion';
import Dialog from './ui/Dialog';
import Button from './ui/Button';
import { StatusPill } from './ui/Status';

function Metric({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="bg-surface-2 p-4 rounded-md">
      <dt className="text-xs text-fg-muted mb-1">{label}</dt>
      <dd className="font-mono font-semibold text-fg">{children}</dd>
    </div>
  );
}

function ChartPanel({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section aria-label={title} className="rounded-lg border border-line p-4 mb-6">
      <div className="flex items-center justify-between mb-4 gap-2">
        <h3 className="text-sm font-semibold">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

export default function ISPLinkModal({ link, onClose }: { link: ISPLink; onClose: () => void }) {
  const anim = useChartAnimation();
  const slaQuery = useQuery({
    queryKey: queryKeys.isp.sla(link.id),
    queryFn: ({ signal }) => getISPLinkSLA(link.id, { signal }),
  });
  const timeseriesQuery = useQuery({
    queryKey: queryKeys.isp.timeseries(link.id),
    queryFn: ({ signal }) => getISPLinkTimeseries(link.id, { signal }),
  });
  const loading = slaQuery.isPending || timeseriesQuery.isPending;
  const failure = slaQuery.error ?? timeseriesQuery.error;
  const error = failure ? (failure instanceof Error ? failure.message : 'Failed to load ISP data') : null;
  const sla = slaQuery.data?.data ?? null;
  const timeSeries = timeseriesQuery.data?.data || [];

  const chartData = timeSeries.map((p) => ({
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    latency: p.latencyMs ?? 0,
    jitter: p.jitterMs ?? 0,
    packetLoss: p.packetLoss ?? 0,
    download: p.downloadMbps ?? 0,
    upload: p.uploadMbps ?? 0,
  })).reverse();

  const slaMet = sla?.slaCompliant;
  const slaPill = slaMet === true
    ? <StatusPill status="up">SLA met</StatusPill>
    : slaMet === false
      ? <StatusPill status="down">SLA breached</StatusPill>
      : <StatusPill status="unknown">No SLA</StatusPill>;
  const tooltipProps = { contentStyle: TOOLTIP_STYLE, labelStyle: TOOLTIP_LABEL_STYLE, itemStyle: TOOLTIP_ITEM_STYLE };

  return (
    <Dialog
      open
      onClose={onClose}
      size="xl"
      title={link.name}
      description={<span className="font-mono">{link.provider} · {link.gateway_ip} · #{link.id}</span>}
      headerActions={!loading && !error ? <div className="pt-1">{slaPill}</div> : undefined}
    >
      {loading ? (
        <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading ISP data…</div>
      ) : error ? (
        <div role="alert" className="h-64 flex flex-col items-center justify-center text-sm gap-3">
          <StatusPill status="down" size="md">{error}</StatusPill>
          <Button size="sm" variant="secondary" icon="refresh" onClick={() => { void slaQuery.refetch(); void timeseriesQuery.refetch(); }}>Retry</Button>
        </div>
      ) : (
        <>
          <dl className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
            <div className="bg-surface-2 p-4 rounded-md">
              <dt className="text-xs text-fg-muted mb-1.5">Status</dt>
              <dd><StatusPill status={sla?.latestStatus || 'unknown'} size="md" /></dd>
            </div>
            <Metric label="Uptime">{sla?.actualUptime != null ? `${sla.actualUptime.toFixed(2)}%` : '—'}</Metric>
            <Metric label="Latency">{sla?.avgLatencyMs != null ? `${sla.avgLatencyMs.toFixed(1)}ms` : '—'}</Metric>
            <Metric label="Packet loss">{sla?.avgPacketLoss != null ? `${sla.avgPacketLoss.toFixed(2)}%` : '—'}</Metric>
          </dl>
          <dl className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
            <Metric label="Bandwidth">{link.bandwidth_mbps} Mbps</Metric>
            <Metric label="SLA target">{link.sla_uptime_percent || 99.5}%</Metric>
            <Metric label="Jitter">{sla?.avgJitterMs != null ? `${sla.avgJitterMs.toFixed(1)}ms` : '—'}</Metric>
            <Metric label="Total probes">{sla?.totalProbes ?? 0}</Metric>
          </dl>

          <ChartPanel title="Latency and jitter">
            {chartData.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-fg-muted text-sm">No metrics data yet. Probes run every {link.monitoring_interval_seconds}s.</div>
            ) : (
              <ResponsiveContainer width="100%" height={256}>
                <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                  <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                  <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                  <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                  <Tooltip {...tooltipProps} formatter={(value: unknown, name: unknown) => [`${Number(value ?? 0).toFixed(1)}ms`, String(name)]} />
                  <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                  <Line name="Latency" type="monotone" dataKey="latency" stroke={SERIES_COLORS[0]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                  <Line name="Jitter" type="monotone" dataKey="jitter" stroke={SERIES_COLORS[1]} strokeWidth={2} dot={false} activeDot={{ r: 3 }} connectNulls {...anim} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </ChartPanel>

          <ChartPanel title="Packet loss">
            {chartData.length === 0 ? (
              <div className="h-40 flex items-center justify-center text-fg-muted text-sm">No data</div>
            ) : (
              <ResponsiveContainer width="100%" height={160}>
                <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                  <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                  <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                  <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}%`} domain={[0, 'auto']} />
                  <Tooltip {...tooltipProps} formatter={(value: unknown) => [`${Number(value ?? 0).toFixed(2)}%`, 'Packet loss']} />
                  <Line name="Packet loss" type="monotone" dataKey="packetLoss" stroke={SERIES_COLORS[2]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </ChartPanel>

          {link.bandwidth_mbps > 0 && (
            <ChartPanel title="Throughput">
              {chartData.length === 0 ? (
                <div className="h-40 flex items-center justify-center text-fg-muted text-sm">No data</div>
              ) : (
                <ResponsiveContainer width="100%" height={160}>
                  <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                    <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                    <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                    <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}M`} />
                    <Tooltip {...tooltipProps} formatter={(value: unknown, name: unknown) => [formatMbps(Number(value ?? 0)), String(name)]} />
                    <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                    <Line name="Download" type="monotone" dataKey="download" stroke={SERIES_COLORS[0]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                    <Line name="Upload" type="monotone" dataKey="upload" stroke={SERIES_COLORS[1]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </ChartPanel>
          )}

          {sla && sla.slaTarget != null && (
            <section aria-label="SLA compliance" className="rounded-lg border border-line p-4">
              <div className="flex items-center justify-between mb-3 gap-2">
                <h3 className="text-sm font-semibold">SLA compliance</h3>
                {slaPill}
              </div>
              <dl className="grid grid-cols-2 gap-4">
                <div>
                  <dt className="text-xs text-fg-muted">Target uptime</dt>
                  <dd className="font-mono text-lg font-semibold text-fg">{sla.slaTarget}%</dd>
                </div>
                <div>
                  <dt className="text-xs text-fg-muted">Actual uptime</dt>
                  <dd className="font-mono text-lg font-semibold text-fg">{sla.actualUptime.toFixed(2)}%</dd>
                </div>
                {sla.slaGap != null && (
                  <>
                    <div>
                      <dt className="text-xs text-fg-muted">Gap</dt>
                      <dd className="mt-1">
                        <StatusPill status={sla.slaGap >= 0 ? 'up' : 'down'}>{sla.slaGap >= 0 ? '+' : ''}{sla.slaGap.toFixed(2)}%</StatusPill>
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-fg-muted">Monthly cost</dt>
                      <dd className="font-mono text-lg font-semibold text-fg">{link.cost_monthly > 0 ? `₹${link.cost_monthly.toLocaleString()}` : '—'}</dd>
                    </div>
                  </>
                )}
              </dl>
            </section>
          )}
        </>
      )}
    </Dialog>
  );
}
