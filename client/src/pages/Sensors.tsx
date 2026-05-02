import { useState, useEffect, useCallback } from 'react';
import {
  BarChart, Bar, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend, Cell,
  RadarChart, PolarGrid, PolarAngleAxis, Radar,
} from 'recharts';
import { getLatestMetrics } from '../api/client';
import type { Metric } from '../api/types';

function statusColor(status: string) {
  if (status === 'down') return 'text-error';
  if (status === 'warning' || status === 'degraded') return 'text-amber-400';
  return 'text-primary';
}

function borderColor(status: string) {
  if (status === 'down') return 'border-error';
  if (status === 'warning' || status === 'degraded') return 'border-amber-500';
  return 'border-primary';
}

function iconFor(protocol: string) {
  if (protocol === 'ping' || protocol === 'icmp') return 'speed';
  if (protocol === 'http' || protocol === 'https') return 'public';
  if (protocol === 'port' || protocol === 'tcp') return 'hub';
  if (protocol === 'system') return 'data_usage';
  return 'sensors';
}

function formatMessage(message: string, protocol: string): string {
  if (!message) return '-';
  if (protocol === 'system') {
    try {
      const info = JSON.parse(message);
      const parts: string[] = [];
      if (info.cpu) parts.push(`CPU ${info.cpu.usage}%`);
      if (info.memory) parts.push(`Mem ${info.memory.percent}%`);
      if (info.disk) parts.push(`Disk ${info.disk.percent}%`);
      if (parts.length > 0) return parts.join(' | ');
    } catch { /* not json */ }
  }
  return message;
}

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

const KNOWN_PROTOCOLS = ['ping', 'http', 'https', 'port', 'system'];

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
    const group = metrics.filter((m) => m.protocol === proto && m.response_time != null);
    const avg = group.length
      ? Math.round(group.reduce((s, m) => s + (m.response_time || 0), 0) / group.length)
      : 0;
    return { subject: proto.toUpperCase(), value: avg, fullMark: 2000 };
  });
}

export default function Sensors() {
  const [metrics, setMetrics] = useState<Metric[]>([]);

  const load = useCallback(async () => {
    const res = await getLatestMetrics();
    setMetrics((res.data || []).slice(0, 120));
  }, []);

  useEffect(() => { load(); }, [load]);

  const total = metrics.length;
  const healthy = metrics.filter((m) => m.status === 'up' || m.status === 'ok').length;
  const warn = metrics.filter((m) => m.status === 'warning' || m.status === 'degraded').length;
  const down = metrics.filter((m) => m.status === 'down').length;
  const healthPercent = total > 0 ? ((healthy / total) * 100).toFixed(1) : '0';

  const protocolBarData = buildProtocolBarData(metrics);
  const activeProtocols = Array.from(new Set(metrics.map((m) => m.protocol))).filter(Boolean);
  const radarData = buildAvgResponseRadar(metrics, activeProtocols);

  return (
    <div>
      {/* Header */}
      <header className="mb-12 flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline text-4xl font-black text-on-surface tracking-tight uppercase mb-2">Monitors &amp; Sensors</h1>
          <p className="text-outline font-label max-w-xl">Real-time surveillance of network vital signs. All protocols operating.</p>
        </div>
        <div className="flex gap-4">
          <div className="bg-surface-container-low px-6 py-3 rounded-xl border border-outline-variant/10 flex items-center gap-4">
            <div className="text-right">
              <p className="text-[10px] uppercase tracking-widest text-outline">System Health</p>
              <p className="text-primary font-bold">{healthPercent}%</p>
            </div>
            <div className="w-12 h-1 bg-surface-container-highest rounded-full overflow-hidden">
              <div className="h-full bg-primary transition-all" style={{ width: `${healthPercent}%` }} />
            </div>
          </div>
        </div>
      </header>

      {/* Status Grid */}
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

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-8">
        {/* Protocol Health Stacked Bar — 2/3 */}
        <div className="xl:col-span-2 bg-surface-container-low rounded-xl p-4 border border-outline-variant/10">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Protocol Health Breakdown</h3>
          {protocolBarData.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-16">No data yet</p>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={protocolBarData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }} barSize={32}>
                <XAxis dataKey="protocol" tick={{ fill: '#8a8a78', fontSize: 11 }} tickLine={false} axisLine={false} />
                <YAxis tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} allowDecimals={false} />
                <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
                <Legend wrapperStyle={{ fontSize: 11, paddingTop: 8 }} formatter={(v) => <span style={{ color: '#c8c5b0' }}>{v}</span>} />
                <Bar dataKey="Healthy" stackId="a" fill="#d9fd3a" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Warning" stackId="a" fill="#f59e0b" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Down" stackId="a" fill="#ff4444" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Avg Response Radar — 1/3 */}
        <div className="bg-surface-container-low rounded-xl p-4 border border-outline-variant/10">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Avg Response (ms) by Protocol</h3>
          {radarData.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-16">No data yet</p>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <RadarChart data={radarData} margin={{ top: 10, right: 20, left: 20, bottom: 10 }}>
                <PolarGrid stroke="#494840" />
                <PolarAngleAxis dataKey="subject" tick={{ fill: '#8a8a78', fontSize: 10 }} />
                <Radar name="Avg ms" dataKey="value" stroke="#d9fd3a" fill="#d9fd3a" fillOpacity={0.2} strokeWidth={2} />
                <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: number) => [`${v}ms`, 'Avg Response']} />
              </RadarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Chart + Feed */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-8">
        <div className="xl:col-span-2 space-y-6">
          {/* Active Sensor Feed */}
          <h2 className="font-headline text-xl font-bold uppercase tracking-tight px-2">Active Sensor Feed</h2>
          <div className="space-y-3">
            {metrics.map((m, i) => (
              <div key={m.id || i} className={`bg-surface-container-low p-5 rounded-xl border-l-4 ${borderColor(m.status)} group hover:bg-surface-container-high transition-all`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-5">
                    <div className={`w-10 h-10 rounded-lg ${m.status === 'down' ? 'bg-error/10' : 'bg-surface-container-highest'} flex items-center justify-center`}>
                      <span className={`material-symbols-outlined ${statusColor(m.status)}`}>{iconFor(m.protocol)}</span>
                    </div>
                    <div>
                      <p className="font-bold text-on-surface tracking-tight">{m.device_name}</p>
                      <div className="flex gap-3 mt-1">
                        <span className="text-[10px] text-outline font-label flex items-center gap-1">
                          <span className="material-symbols-outlined text-[14px]">schedule</span>
                          {new Date(m.timestamp || m.created_at).toLocaleTimeString()}
                        </span>
                        <span className="text-[10px] text-outline font-label flex items-center gap-1">
                          <span className="material-symbols-outlined text-[14px]">lan</span>
                          {m.protocol.toUpperCase()}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <p className={`text-xl font-headline font-bold ${statusColor(m.status)} tracking-tighter`}>
                      {m.response_time != null ? `${m.response_time}ms` : m.status.toUpperCase()}
                    </p>
                    <p className="text-[10px] text-outline uppercase font-label max-w-xs truncate">{formatMessage(m.message, m.protocol)}</p>
                  </div>
                </div>
              </div>
            ))}
            {metrics.length === 0 && <p className="text-sm text-on-surface-variant text-center py-8">No sensor data yet</p>}
          </div>
        </div>

        {/* Sidebar */}
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
                      <div className="h-1.5 rounded-full bg-primary transition-all" style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
