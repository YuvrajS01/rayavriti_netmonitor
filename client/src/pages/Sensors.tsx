import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  RadarChart, PolarGrid, PolarAngleAxis, Radar,
} from 'recharts';
import { getLatestMetrics } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { Metric } from '../api/types';
import { statusTextColor, statusBorderColor } from '../utils/colors';
import { sensorIconForProtocol } from '../utils/icons';
import { formatMetricDetails } from '../utils/formatters';
import { TOOLTIP_STYLE, AXIS_TICK_STYLE, LEGEND_STYLE, legendFormatter } from '../utils/chartConfig';
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
      Healthy: group.filter((m) => m.status === 'up' || m.status === 'ok').length,
      Warning: group.filter((m) => m.status === 'warning' || m.status === 'degraded').length,
      Down: group.filter((m) => m.status === 'down').length,
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
  const healthy = useMemo(() => metrics.filter((m) => m.status === 'up' || m.status === 'ok').length, [metrics]);
  const warn = useMemo(() => metrics.filter((m) => m.status === 'warning' || m.status === 'degraded').length, [metrics]);
  const down = useMemo(() => metrics.filter((m) => m.status === 'down').length, [metrics]);
  const healthPercent = total > 0 ? ((healthy / total) * 100).toFixed(1) : '0';

  const protocolBarData = useMemo(() => buildProtocolBarData(metrics), [metrics]);
  const activeProtocols = useMemo(() => Array.from(new Set(metrics.map((m) => m.protocol))).filter(Boolean), [metrics]);
  const radarData = useMemo(() => buildAvgResponseRadar(metrics, activeProtocols), [metrics, activeProtocols]);
  const [visibleCount, setVisibleCount] = useState(20);
  const visibleMetrics = metrics.slice(0, visibleCount);

  return (
    <div>
      <SectionHeader
        title="Monitors & Sensors"
        subtitle="Monitor sensor health, protocol distribution, and response times."
        action={
          <div className="bg-surface-1 px-6 py-3 rounded-lg border border-line flex items-center gap-4">
            <div className="text-right">
              <p className="text-xs uppercase tracking-wide text-fg-subtle">System Health</p>
              <p className="text-primary font-semibold">{healthPercent}%</p>
            </div>
            <div className="w-12 h-1 bg-inset rounded-full overflow-hidden">
              <div className="h-full bg-primary transition-[width]" style={{ width: `${healthPercent}%` }} />
            </div>
          </div>
        }
      />

      {loading && <LoadingState message="Loading sensor data..." />}

      {error && !loading && <ErrorState message={error} onRetry={load} />}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
            <StatCard label="Total Sensors" value={total} icon="sensors" trend="up" sparklineData={metrics.map((m) => m.responseTime ?? 0)} />
            <StatCard label="Healthy" value={healthy} icon="check_circle" color="text-primary" trend="up" sparklineData={metrics.map((m) => (m.status === 'up' || m.status === 'ok' ? 1 : 0))} />
            <StatCard label="Warning" value={warn} icon="warning" color="text-warning" trend={warn ? 'flat' : 'down'} sparklineData={metrics.map((m) => (m.status === 'warning' || m.status === 'degraded' ? 1 : 0))} />
            <StatCard label="Down" value={down} icon="error" color="text-error" trend={down ? 'up' : 'down'} sparklineData={metrics.map((m) => (m.status === 'down' ? 1 : 0))} />
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
            <div className="xl:col-span-2 bg-surface-1 rounded-lg p-4 border border-line">
              <h3 className="text-sm font-sans font-semibold mb-3 uppercase tracking-wide">Protocol Health Breakdown</h3>
              {protocolBarData.length === 0 ? (
                <p className="text-xs text-fg-muted text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={protocolBarData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }} barSize={32}>
                    <XAxis dataKey="protocol" tick={{ fill: '#77766d', fontSize: 11 }} tickLine={false} axisLine={false} />
                    <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                    <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                    <Bar dataKey="Healthy" stackId="a" fill="#d9fd3a" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="Warning" stackId="a" fill="#e5a910" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="Down" stackId="a" fill="#ff7351" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-surface-1 rounded-lg p-4 border border-line">
              <h3 className="text-sm font-sans font-semibold mb-3 uppercase tracking-wide">Avg Response (ms) by Protocol</h3>
              {radarData.length === 0 ? (
                <p className="text-xs text-fg-muted text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <RadarChart data={radarData} margin={{ top: 10, right: 20, left: 20, bottom: 10 }}>
                    <PolarGrid stroke="var(--color-line)" />
                    <PolarAngleAxis dataKey="subject" tick={{ fill: '#77766d', fontSize: 10 }} />
                    <Radar name="Avg ms" dataKey="value" stroke="#d9fd3a" fill="#d9fd3a" fillOpacity={0.2} strokeWidth={2} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}ms`, 'Avg Response']} />
                  </RadarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-8">
            <div className="xl:col-span-2 space-y-6">
              <h2 className="font-sans text-xl font-semibold uppercase tracking-tight px-2">Active Sensor Feed</h2>
              <div className="space-y-3">
                {visibleMetrics.map((m, i) => (
                   <div key={m.id || i} className={`bg-surface-1 p-5 rounded-lg border ${statusBorderColor(m.status)} group hover:bg-surface-1 transition-colors`}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-5">
                        <div className={`w-10 h-10 rounded-lg ${m.status === 'down' ? 'bg-error/10' : 'bg-inset'} flex items-center justify-center relative`}>
                          <span className={`material-symbols-outlined ${statusTextColor(m.status)}`}>{sensorIconForProtocol(m.protocol)}</span>
                          <span className={`absolute -top-1 -right-1 w-3 h-3 rounded-full border-2 border-surface-1 ${m.status === 'down' ? 'bg-error glow-error' : m.status === 'warning' || m.status === 'degraded' ? 'bg-warning' : 'bg-success status-dot-live'}`} />
                        </div>
                        <div>
                          <p className="font-semibold text-fg tracking-tight">{m.deviceName}</p>
                          <div className="flex gap-3 mt-1">
                            <span className="text-xs text-fg-subtle font-sans flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]">schedule</span>
                              {new Date(m.timestamp || m.createdAt).toLocaleTimeString()}
                            </span>
                            <span className="text-xs text-fg-subtle font-sans flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]">lan</span>
                              {m.protocol?.toUpperCase()}
                            </span>
                          </div>
                        </div>
                      </div>
                      <div className="text-right">
                        <p className={`text-xl font-sans font-semibold ${statusTextColor(m.status)} tracking-tighter`}>
                          {m.responseTime != null ? `${m.responseTime}ms` : (m.status ?? 'UNKNOWN').toUpperCase()}
                        </p>
                        <p className="text-xs text-fg-subtle uppercase font-sans max-w-xs truncate">{formatMetricDetails(m.details, m.protocol)}</p>
                      </div>
                    </div>
                  </div>
                ))}
                {metrics.length === 0 && <p className="text-sm text-fg-muted text-center py-8">No sensor data yet</p>}
                {visibleCount < metrics.length && (
                  <button
                    onClick={() => setVisibleCount((prev) => prev + 20)}
                    className="w-full py-3 text-xs font-semibold uppercase tracking-wide text-fg-muted hover:text-fg border border-line rounded-lg hover:border-line-strong/40 transition-colors"
                  >
                    Show more ({metrics.length - visibleCount} remaining)
                  </button>
                )}
              </div>
            </div>

            <div className="space-y-8">
              <div className="bg-surface-1 p-6 rounded-lg border border-line">
                <h3 className="font-sans font-semibold uppercase text-xs tracking-wide text-fg mb-6">Protocol Summary</h3>
                <div className="space-y-4 font-sans">
                  {KNOWN_PROTOCOLS.map((proto) => {
                    const count = metrics.filter((m) => m.protocol === proto).length;
                    if (count === 0) return null;
                    const h = metrics.filter((m) => m.protocol === proto && (m.status === 'up' || m.status === 'ok')).length;
                    const pct = Math.round((h / count) * 100);
                    return (
                      <div key={proto}>
                        <div className="flex justify-between items-center mb-1">
                          <span className="text-xs uppercase tracking-wide text-fg-muted">{proto}</span>
                          <span className="text-xs font-semibold text-primary">{h}/{count}</span>
                        </div>
                        <div className="h-1.5 bg-inset rounded-full">
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
