import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend, CartesianGrid,
  RadarChart, PolarGrid, PolarAngleAxis, Radar,
} from 'recharts';
import { getLatestMetrics } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { Metric } from '../api/types';
import { normalizeStatus, statusColorVar, statusMeta } from '../utils/status';
import { useChartAnimation } from '../hooks/useReducedMotion';
import { StatusDot } from '../components/ui/Status';
import { sensorIconForProtocol } from '../utils/icons';
import { formatMetricDetails } from '../utils/formatters';
import { TOOLTIP_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_ITEM_STYLE, AXIS_TICK_STYLE, GRID_COLOR, LEGEND_STYLE, SERIES_COLORS, legendFormatter } from '../utils/chartConfig';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';

const KNOWN_PROTOCOLS = ['ping', 'http', 'https', 'port', 'system', 'snmp'];

interface ProtocolBarPoint {
  protocol: string;
  Healthy: number;
  Warning: number;
  Down: number;
}

function buildProtocolBarData(metrics: Metric[]): ProtocolBarPoint[] {
  const protos = Array.from(new Set(metrics.map((m) => m.protocol))).filter(Boolean);
  return protos.map((proto) => {
    const group = metrics.filter((m) => m.protocol === proto);
    return {
      protocol: proto.toUpperCase(),
      Healthy: group.filter((m) => normalizeStatus(m.status) === 'up').length,
      Warning: group.filter((m) => normalizeStatus(m.status) === 'warning').length,
      Down: group.filter((m) => normalizeStatus(m.status) === 'down').length,
    };
  });
}

interface RadarPoint { subject: string; value: number; fullMark: number }

function buildAvgResponseRadar(metrics: Metric[], protocols: string[]): RadarPoint[] {
  return protocols.map((proto) => {
    const group = metrics.filter((m) => m.protocol === proto && m.responseTime != null);
    const avg = group.length
      ? Math.round(group.reduce((s, m) => s + (m.responseTime || 0), 0) / group.length)
      : 0;
    return { subject: proto.toUpperCase(), value: avg, fullMark: 2000 };
  });
}

export default function Sensors() {
  const anim = useChartAnimation();
  // Shared with Devices/Dashboard and kept live by useLiveQueryBridge.
  const metricsQuery = useQuery({
    queryKey: queryKeys.metrics.latest,
    queryFn: ({ signal }) => getLatestMetrics({ signal }),
  });
  const metrics = useMemo<Metric[]>(() => (metricsQuery.data?.data || []).slice(0, 120), [metricsQuery.data]);
  const loading = metricsQuery.isPending;
  const error = metricsQuery.isError ? 'Failed to load sensor data. Please try again.' : null;
  const load = () => { void metricsQuery.refetch(); };

  const total = metrics.length;
  const healthy = useMemo(() => metrics.filter((m) => normalizeStatus(m.status) === 'up').length, [metrics]);
  const warn = useMemo(() => metrics.filter((m) => normalizeStatus(m.status) === 'warning').length, [metrics]);
  const down = useMemo(() => metrics.filter((m) => normalizeStatus(m.status) === 'down').length, [metrics]);
  const healthPercent = total > 0 ? ((healthy / total) * 100).toFixed(1) : '0';

  const protocolBarData = useMemo(() => buildProtocolBarData(metrics), [metrics]);
  const activeProtocols = useMemo(() => Array.from(new Set(metrics.map((m) => m.protocol))).filter(Boolean), [metrics]);
  const radarData = useMemo(() => buildAvgResponseRadar(metrics, activeProtocols), [metrics, activeProtocols]);
  const [visibleCount, setVisibleCount] = useState(20);
  const visibleMetrics = metrics.slice(0, visibleCount);

  return (
    <div>
      <SectionHeader
        title="Monitors and sensors"
        subtitle="Monitor sensor health, protocol distribution, and response times."
        action={
          <div className="bg-surface-1 px-5 py-3 rounded-lg border border-line flex items-center gap-4">
            <div className="text-right">
              <p className="text-xs text-fg-muted">Sensors up</p>
              <p className="font-semibold tabular-nums text-fg">{healthPercent}%</p>
            </div>
            <div className="w-16 h-1.5 bg-surface-3 rounded-full overflow-hidden" role="meter" aria-label="Sensors up" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Number(healthPercent)}>
              <div className="h-full bg-primary transition-[width]" style={{ width: `${healthPercent}%` }} />
            </div>
          </div>
        }
      />

      {loading && <LoadingState message="Loading sensor data…" />}

      {error && !loading && <ErrorState message={error} onRetry={load} />}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
            <StatCard label="Total sensors" value={total} icon="sensors" />
            <StatCard label="Up" value={healthy} icon={statusMeta('up').icon} color={healthy ? statusMeta('up').text : 'text-fg'} />
            <StatCard label="Warning" value={warn} icon={statusMeta('warning').icon} color={warn ? statusMeta('warning').text : 'text-fg'} />
            <StatCard label="Down" value={down} icon={statusMeta('down').icon} color={down ? statusMeta('down').text : 'text-fg'} />
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
            <div className="xl:col-span-2 bg-surface-1 rounded-lg p-4 border border-line">
              <h3 className="text-sm font-semibold mb-3">Protocol health breakdown</h3>
              {protocolBarData.length === 0 ? (
                <p className="text-sm text-fg-muted text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={protocolBarData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }} barSize={32}>
                    <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                    <XAxis dataKey="protocol" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                    <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} cursor={{ fill: 'var(--color-surface-2)' }} />
                    <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                    {/* These series ARE statuses, so they use the status colours (stacked with a surface gap). */}
                    <Bar dataKey="Healthy" name="Up" stackId="a" fill={statusColorVar('up')} stroke="var(--color-surface-1)" strokeWidth={1} {...anim} />
                    <Bar dataKey="Warning" stackId="a" fill={statusColorVar('warning')} stroke="var(--color-surface-1)" strokeWidth={1} {...anim} />
                    <Bar dataKey="Down" stackId="a" fill={statusColorVar('down')} stroke="var(--color-surface-1)" strokeWidth={1} radius={[4, 4, 0, 0]} {...anim} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-surface-1 rounded-lg p-4 border border-line">
              <h3 className="text-sm font-semibold mb-3">Avg response (ms) by protocol</h3>
              {radarData.length === 0 ? (
                <p className="text-sm text-fg-muted text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <RadarChart data={radarData} margin={{ top: 10, right: 20, left: 20, bottom: 10 }}>
                    <PolarGrid stroke="var(--color-line)" />
                    <PolarAngleAxis dataKey="subject" tick={AXIS_TICK_STYLE} />
                    <Radar name="Avg ms" dataKey="value" stroke={SERIES_COLORS[0]} fill={SERIES_COLORS[0]} fillOpacity={0.2} strokeWidth={2} {...anim} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}ms`, 'Avg response']} />
                  </RadarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-8">
            <div className="xl:col-span-2 space-y-6">
              <h2 className="text-xl font-semibold">Active sensor feed</h2>
              <div className="space-y-3">
                {visibleMetrics.map((m, i) => { const meta = statusMeta(m.status); return (
                   <div key={m.id || i} className={`bg-surface-1 p-5 rounded-lg border border-line border-l-4 ${meta.borderL}`}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-5">
                        <div className="w-10 h-10 rounded-md bg-inset flex items-center justify-center relative">
                          <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">{sensorIconForProtocol(m.protocol)}</span>
                          <StatusDot status={meta} className="absolute -top-1 -right-1 rounded-sm bg-surface-1 p-0.5" />
                        </div>
                        <div>
                          <p className="font-semibold text-fg">{m.deviceName}</p>
                          <div className="flex gap-3 mt-1">
                            <span className="text-xs text-fg-subtle font-mono flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]" aria-hidden="true">schedule</span>
                              {new Date(m.timestamp || m.createdAt).toLocaleTimeString()}
                            </span>
                            <span className="text-xs text-fg-subtle flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]" aria-hidden="true">lan</span>
                              {m.protocol?.toUpperCase()}
                            </span>
                          </div>
                        </div>
                      </div>
                      <div className="text-right">
                        <p className="font-mono text-xl font-semibold text-fg">
                          {m.responseTime != null ? `${m.responseTime}ms` : meta.label}
                        </p>
                        <p className="text-xs text-fg-subtle max-w-xs truncate">{formatMetricDetails(m.details, m.protocol)}</p>
                      </div>
                    </div>
                  </div>
                ); })}
                {metrics.length === 0 && <p className="text-sm text-fg-muted text-center py-8">No sensor data yet</p>}
                {visibleCount < metrics.length && (
                  <button
                    type="button"
                    onClick={() => setVisibleCount((prev) => prev + 20)}
                    className="w-full py-3 text-sm font-medium text-fg-muted hover:text-fg border border-line rounded-lg hover:border-line-strong hover:bg-surface-2 transition-colors"
                  >
                    Show more ({metrics.length - visibleCount} remaining)
                  </button>
                )}
              </div>
            </div>

            <div className="space-y-8">
              <div className="bg-surface-1 p-6 rounded-lg border border-line">
                <h3 className="font-semibold text-sm text-fg mb-1">Protocol summary</h3>
                <p className="text-xs text-fg-muted mb-5">Sensors up per protocol</p>
                <div className="space-y-4">
                  {KNOWN_PROTOCOLS.map((proto) => {
                    const count = metrics.filter((m) => m.protocol === proto).length;
                    if (count === 0) return null;
                    const h = metrics.filter((m) => m.protocol === proto && normalizeStatus(m.status) === 'up').length;
                    const pct = Math.round((h / count) * 100);
                    return (
                      <div key={proto}>
                        <div className="flex justify-between items-center mb-1">
                          <span className="text-xs uppercase text-fg-muted">{proto}</span>
                          <span className="font-mono text-xs font-semibold text-fg">{h}/{count}</span>
                        </div>
                        <div className="h-1.5 bg-surface-3 rounded-full" aria-hidden="true">
                          <div className="h-1.5 rounded-full bg-primary transition-[width]" style={{ width: `${pct}%` }} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
