import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  RadarChart, PolarGrid, PolarAngleAxis, Radar,
} from 'recharts';
import { getLatestMetrics } from '../api/client';
import type { Metric } from '../api/types';
import { statusTextColor, statusBorderColor } from '../utils/colors';
import { sensorIconForProtocol } from '../utils/icons';
import { formatMetricDetails } from '../utils/formatters';
import { TOOLTIP_STYLE, AXIS_TICK_STYLE, LEGEND_STYLE, legendFormatter } from '../utils/chartConfig';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';
import SectionHeader from '../components/ui/SectionHeader';

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
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setError(null);
      const res = await getLatestMetrics();
      setMetrics((res.data || []).slice(0, 120));
    } catch {
      setError('Failed to load sensor data. Please try again.');
    } finally {
      setLoading(false);
    }
  }, []);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { load(); }, [load]);

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
        subtitle="Real-time surveillance of network vital signs. All protocols operating."
        action={
          <div className="bg-surface-container-low px-6 py-3 rounded-xl border border-outline-variant/10 flex items-center gap-4">
            <div className="text-right">
              <p className="text-[10px] uppercase tracking-widest text-outline">System Health</p>
              <p className="text-primary font-bold">{healthPercent}%</p>
            </div>
            <div className="w-12 h-1 bg-surface-container-highest rounded-full overflow-hidden">
              <div className="h-full bg-primary transition-[width]" style={{ width: `${healthPercent}%` }} />
            </div>
          </div>
        }
      />

      {loading && <LoadingState message="Loading sensor data..." />}

      {error && !loading && <ErrorState message={error} onRetry={load} />}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-12">
            <div className="bg-surface-container-low p-6 rounded-xl border border-outline-variant/10 ambient-glow-primary">
              <div className="flex justify-between items-start mb-4">
                <span className="material-symbols-outlined text-primary bg-primary/10 p-2 rounded-lg">sensors</span>
                <span className="text-[10px] text-primary bg-primary/10 px-2 py-0.5 rounded-full font-bold">TOTAL</span>
              </div>
              <h3 className="text-outline font-label uppercase tracking-widest text-[10px] mb-1">Total Sensors</h3>
              <span className="text-3xl font-headline font-bold text-on-surface">{total}</span>
            </div>
            <div className="bg-surface-container-low p-6 rounded-xl border border-outline-variant/10">
              <div className="flex justify-between items-start mb-4">
                <span className="material-symbols-outlined text-primary bg-primary/10 p-2 rounded-lg">check_circle</span>
                <span className="text-[10px] text-primary bg-primary/10 px-2 py-0.5 rounded-full font-bold">HEALTHY</span>
              </div>
              <h3 className="text-outline font-label uppercase tracking-widest text-[10px] mb-1">Healthy</h3>
              <span className="text-3xl font-headline font-bold text-primary">{healthy}</span>
            </div>
            <div className="bg-surface-container-low p-6 rounded-xl border border-outline-variant/10">
              <div className="flex justify-between items-start mb-4">
                <span className="material-symbols-outlined text-amber-400 bg-amber-400/10 p-2 rounded-lg">warning</span>
                <span className="text-[10px] text-amber-400 bg-amber-400/10 px-2 py-0.5 rounded-full font-bold">WARNING</span>
              </div>
              <h3 className="text-outline font-label uppercase tracking-widest text-[10px] mb-1">Warning</h3>
              <span className="text-3xl font-headline font-bold text-amber-400">{warn}</span>
            </div>
            <div className="bg-surface-container-low p-6 rounded-xl border border-outline-variant/10">
              <div className="flex justify-between items-start mb-4">
                <span className="material-symbols-outlined text-error bg-error/10 p-2 rounded-lg">error</span>
                <span className="text-[10px] text-error bg-error/10 px-2 py-0.5 rounded-full font-bold">DOWN</span>
              </div>
              <h3 className="text-outline font-label uppercase tracking-widest text-[10px] mb-1">Down</h3>
              <span className="text-3xl font-headline font-bold text-error">{down}</span>
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-8">
            <div className="xl:col-span-2 bg-surface-container-low rounded-xl p-4 border border-outline-variant/10">
              <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Protocol Health Breakdown</h3>
              {protocolBarData.length === 0 ? (
                <p className="text-xs text-on-surface-variant text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={protocolBarData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }} barSize={32}>
                    <XAxis dataKey="protocol" tick={{ fill: '#8a8a78', fontSize: 11 }} tickLine={false} axisLine={false} />
                    <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} allowDecimals={false} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                    <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                    <Bar dataKey="Healthy" stackId="a" fill="#d9fd3a" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="Warning" stackId="a" fill="#f59e0b" radius={[0, 0, 0, 0]} />
                    <Bar dataKey="Down" stackId="a" fill="#ff4444" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-surface-container-low rounded-xl p-4 border border-outline-variant/10">
              <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Avg Response (ms) by Protocol</h3>
              {radarData.length === 0 ? (
                <p className="text-xs text-on-surface-variant text-center py-16">No data yet</p>
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <RadarChart data={radarData} margin={{ top: 10, right: 20, left: 20, bottom: 10 }}>
                    <PolarGrid stroke="var(--color-outline-variant)" />
                    <PolarAngleAxis dataKey="subject" tick={{ fill: '#8a8a78', fontSize: 10 }} />
                    <Radar name="Avg ms" dataKey="value" stroke="#d9fd3a" fill="#d9fd3a" fillOpacity={0.2} strokeWidth={2} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}ms`, 'Avg Response']} />
                  </RadarChart>
                </ResponsiveContainer>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-8">
            <div className="xl:col-span-2 space-y-6">
              <h2 className="font-headline text-xl font-bold uppercase tracking-tight px-2">Active Sensor Feed</h2>
              <div className="space-y-3">
                {visibleMetrics.map((m, i) => (
                  <div key={m.id || i} className={`bg-surface-container-low p-5 rounded-xl border-l-4 ${statusBorderColor(m.status)} group hover:bg-surface-container-high transition-[background-color]`}>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-5">
                        <div className={`w-10 h-10 rounded-lg ${m.status === 'down' ? 'bg-error/10' : 'bg-surface-container-highest'} flex items-center justify-center`}>
                          <span className={`material-symbols-outlined ${statusTextColor(m.status)}`}>{sensorIconForProtocol(m.protocol)}</span>
                        </div>
                        <div>
                          <p className="font-bold text-on-surface tracking-tight">{m.deviceName}</p>
                          <div className="flex gap-3 mt-1">
                            <span className="text-[10px] text-outline font-label flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]">schedule</span>
                              {new Date(m.timestamp || m.createdAt).toLocaleTimeString()}
                            </span>
                            <span className="text-[10px] text-outline font-label flex items-center gap-1">
                              <span className="material-symbols-outlined text-[14px]">lan</span>
                              {m.protocol.toUpperCase()}
                            </span>
                          </div>
                        </div>
                      </div>
                      <div className="text-right">
                        <p className={`text-xl font-headline font-bold ${statusTextColor(m.status)} tracking-tighter`}>
                          {m.responseTime != null ? `${m.responseTime}ms` : m.status.toUpperCase()}
                        </p>
                        <p className="text-[10px] text-outline uppercase font-label max-w-xs truncate">{formatMetricDetails(m.details, m.protocol)}</p>
                      </div>
                    </div>
                  </div>
                ))}
                {metrics.length === 0 && <p className="text-sm text-on-surface-variant text-center py-8">No sensor data yet</p>}
                {visibleCount < metrics.length && (
                  <button
                    onClick={() => setVisibleCount((prev) => prev + 20)}
                    className="w-full py-3 text-xs font-bold uppercase tracking-widest text-on-surface-variant hover:text-primary border border-outline-variant/20 rounded-lg hover:border-primary/40 transition-colors"
                  >
                    Show more ({metrics.length - visibleCount} remaining)
                  </button>
                )}
              </div>
            </div>

            <div className="space-y-8">
              <div className="bg-surface-container-low p-6 rounded-xl border border-outline-variant/10">
                <h3 className="font-headline font-bold uppercase text-xs tracking-widest text-on-surface mb-6">Protocol Summary</h3>
                <div className="space-y-4 font-label">
                  {KNOWN_PROTOCOLS.map((proto) => {
                    const count = metrics.filter((m) => m.protocol === proto).length;
                    if (count === 0) return null;
                    const h = metrics.filter((m) => m.protocol === proto && (m.status === 'up' || m.status === 'ok')).length;
                    const pct = Math.round((h / count) * 100);
                    return (
                      <div key={proto}>
                        <div className="flex justify-between items-center mb-1">
                          <span className="text-xs uppercase tracking-widest text-on-surface-variant">{proto}</span>
                          <span className="text-xs font-bold text-primary">{h}/{count}</span>
                        </div>
                        <div className="h-1.5 bg-surface-container-highest rounded-full">
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
