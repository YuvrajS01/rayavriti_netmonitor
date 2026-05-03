import { useState, useEffect, useCallback } from 'react';
import {
  LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  PieChart, Pie, Cell,
} from 'recharts';
import { getStats, getLatestMetrics, getAlerts } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, Metric, Alert, SystemInfo } from '../api/types';
import ExpandedChartsModal from '../components/ExpandedChartsModal';
import ResourceLoadModal from '../components/ResourceLoadModal';

function StatCard({ label, value, color = 'text-primary' }: { label: string; value: string | number; color?: string }) {
  return (
    <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-primary/30">
      <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">{label}</p>
      <p className={`font-headline text-3xl font-bold ${color}`}>{value}</p>
    </div>
  );
}

function ResourceBar({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span>{label}</span>
        <span>{value}%</span>
      </div>
      <div className="h-2 bg-surface-container-highest rounded">
        <div className="h-2 rounded transition-all duration-500" style={{ width: `${Math.min(100, value)}%`, background: color }} />
      </div>
    </div>
  );
}

// 6 distinct neon-ish colors for multi-device lines
const DEVICE_COLORS = ['#d9fd3a', '#ff7351', '#6ee7f7', '#c084fc', '#4ade80', '#fb923c'];

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

interface MultiLinePoint {
  time: string;
  [deviceName: string]: string | number;
}

/** Build a per-device multi-line dataset from latest metrics list.
 *  We use the metrics array which is ordered newest-first; we reverse it for the chart. */
function buildMultiLineData(metrics: Metric[]): { data: MultiLinePoint[]; devices: string[] } {
  // Group metrics by device, keeping only the last N readings per device
  const byDevice = new Map<string, Metric[]>();
  for (const m of metrics) {
    const key = m.device_name || `Device ${m.device_id}`;
    if (!byDevice.has(key)) byDevice.set(key, []);
    byDevice.get(key)!.push(m);
  }

  // Take top 6 devices by recency
  const devices = Array.from(byDevice.keys()).slice(0, 6);

  // Build a unified time axis: take up to 20 time ticks from the first device
  const primary = byDevice.get(devices[0]) ?? [];
  const timeSlots = primary
    .slice(0, 20)
    .reverse()
    .map((m) => new Date(m.timestamp || m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }));

  const data: MultiLinePoint[] = timeSlots.map((time, idx) => {
    const point: MultiLinePoint = { time };
    for (const dev of devices) {
      const devMetrics = byDevice.get(dev) ?? [];
      const reversed = [...devMetrics].slice(0, 20).reverse();
      const m = reversed[idx];
      if (m) point[dev] = m.response_time ?? 0;
    }
    return point;
  });

  return { data, devices };
}

const STATUS_COLORS: Record<string, string> = {
  up: '#d9fd3a',
  ok: '#d9fd3a',
  warning: '#f59e0b',
  degraded: '#f59e0b',
  down: '#ff4444',
  unknown: '#6b7280',
};

const STATUS_LABELS: Record<string, string> = {
  up: 'Healthy',
  warning: 'Warning',
  down: 'Down',
  unknown: 'Unknown',
};

interface DonutSlice { name: string; value: number; color: string }

function buildDonutData(metrics: Metric[]): DonutSlice[] {
  // Count unique devices (latest metric per device)
  const byDevice = new Map<number, string>();
  for (const m of metrics) byDevice.set(m.device_id, m.status);

  const counts: Record<string, number> = { up: 0, warning: 0, down: 0, unknown: 0 };
  for (const [, status] of byDevice) {
    if (status === 'up' || status === 'ok') counts.up++;
    else if (status === 'warning' || status === 'degraded') counts.warning++;
    else if (status === 'down') counts.down++;
    else counts.unknown++;
  }

  return Object.entries(counts)
    .filter(([, v]) => v > 0)
    .map(([name, value]) => ({
      name: STATUS_LABELS[name] ?? name,
      value,
      color: STATUS_COLORS[name] ?? '#6b7280',
    }));
}

/* Custom label for the donut centre */
function DonutCenter({ cx, cy, total }: { cx: number; cy: number; total: number }) {
  return (
    <text x={cx} y={cy} textAnchor="middle" dominantBaseline="middle" fill="#f4f1e6">
      <tspan x={cx} dy="-0.4em" fontSize="22" fontWeight="bold" fontFamily="'Space Grotesk', sans-serif">{total}</tspan>
      <tspan x={cx} dy="1.4em" fontSize="10" fill="#8a8a78" fontFamily="'Space Grotesk', sans-serif">DEVICES</tspan>
    </text>
  );
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({ totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 0, activeAlerts: 0, avgResponseTime: 0 });
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [historyMetrics, setHistoryMetrics] = useState<Metric[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [lastUpdated, setLastUpdated] = useState('Waiting for updates...');
  const [systemInfo, setSystemInfo] = useState<{ cpu: number; memory: number; errorRate: number; raw?: SystemInfo }>({ cpu: 0, memory: 0, errorRate: 0 });
  const [showExpandedCharts, setShowExpandedCharts] = useState(false);
  const [showResourceModal, setShowResourceModal] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [statsRes, metricsRes, alertsRes] = await Promise.all([
        getStats(), getLatestMetrics(), getAlerts('active'),
      ]);
      setStats(statsRes.data);
      setMetrics(metricsRes.data || []);
      setHistoryMetrics(metricsRes.data || []);
      setAlerts(alertsRes.data || []);
      computeSystemInfo(metricsRes.data || []);
    } catch { /* handled by interceptor */ }
  }, []);

  const computeSystemInfo = (m: Metric[]) => {
    const latest = m.slice(0, 40);
    const total = latest.length || 1;
    const down = latest.filter((x) => x.status === 'down').length;
    const warn = latest.filter((x) => x.status === 'warning' || x.status === 'degraded').length;
    const avgResp = latest.reduce((s, x) => s + (x.response_time || 0), 0) / total;
    const systemMetric = latest.find((x) => x.protocol === 'system') || latest.find((x) => x.protocol === 'snmp');
    let cpu = Math.min(95, Math.round(avgResp / 6 + warn * 4));
    let memory = Math.min(95, Math.round(avgResp / 7 + down * 8 + 28));
    let raw = undefined;
    if (systemMetric?.message) {
      try {
        const info = JSON.parse(systemMetric.message);
        raw = info;
        if (info.cpu) cpu = Math.round(info.cpu.usage);
        if (info.memory) memory = Math.round(info.memory.percent);
      } catch { /* not JSON, use estimates */ }
    }
    setSystemInfo({
      cpu,
      memory,
      errorRate: Math.min(100, Math.round((down / total) * 100)),
      raw
    });
  };

  useEffect(() => { loadData(); }, [loadData]);

  useSocket({
    onBootstrap: (payload) => {
      const p = payload as { stats: DashboardStats; latestMetrics: Metric[]; alerts: Alert[] };
      if (p.stats) setStats(p.stats);
      if (p.latestMetrics) { 
        setMetrics(p.latestMetrics); 
        setHistoryMetrics(p.latestMetrics);
        computeSystemInfo(p.latestMetrics); 
      }
      if (p.alerts) setAlerts(p.alerts);
    },
    onMetricUpdate: (metric) => {
      setMetrics((prev) => {
        const m = metric as unknown as Metric;
        const updated = [m, ...prev.filter((x) => x.device_id !== m.device_id)];
        computeSystemInfo(updated);
        return updated;
      });
      setHistoryMetrics((prev) => {
        const m = metric as unknown as Metric;
        const updated = [m, ...prev];
        if (updated.length > 500) updated.pop();
        return updated;
      });
      setLastUpdated(`Updated ${new Date().toLocaleTimeString()}`);
      getStats().then((r) => setStats(r.data)).catch(() => {});
    },
    onAlertTriggered: () => {
      Promise.all([getAlerts('active'), getStats()]).then(([a, s]) => {
        setAlerts(a.data || []);
        setStats(s.data);
      }).catch(() => {});
    },
  });

  const { data: multiLineData, devices: trackedDevices } = buildMultiLineData(historyMetrics);
  const donutData = buildDonutData(metrics);
  const donutTotal = donutData.reduce((s, d) => s + d.value, 0);

  return (
    <div>
      {/* Header */}
      <header className="mb-12 flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">Network Overview</h1>
          <p className="text-on-surface-variant font-body max-w-xl">Real-time surveillance dashboard. All systems monitored and reporting.</p>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-primary animate-pulse" />
            <span className="text-primary font-mono text-xs">{lastUpdated}</span>
          </div>
        </div>
      </header>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-12">
        <StatCard label="Total Devices" value={stats.totalDevices} />
        <StatCard label="Online" value={stats.onlineDevices} />
        <StatCard label="Uptime" value={`${stats.uptimePercent}%`} />
        <StatCard label="Active Alerts" value={stats.activeAlerts} color="text-error" />
      </div>

      {/* Charts Row 1: Multi-device response timeline + Status Distribution */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        {/* Multi-device Response Timeline — takes 2/3 width */}
        <div 
          className="xl:col-span-2 bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 hover:border-primary/50 hover:shadow-[0_0_15px_rgba(217,253,58,0.1)] transition-all cursor-pointer group"
          onClick={() => setShowExpandedCharts(true)}
        >
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest group-hover:text-primary transition-colors">Response Time per Device</h3>
            <span className="material-symbols-outlined text-on-surface-variant group-hover:text-primary text-sm transition-colors">open_in_full</span>
          </div>
          {trackedDevices.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-16">No device metrics yet</p>
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <LineChart data={multiLineData} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
                <XAxis
                  dataKey="time"
                  tick={{ fill: '#8a8a78', fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fill: '#8a8a78', fontSize: 10 }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => `${v}ms`}
                  width={48}
                />
                <Tooltip
                  contentStyle={TOOLTIP_STYLE}
                  formatter={(value: unknown, name: unknown) => [`${Number(value ?? 0)}ms`, String(name)]}
                />
                <Legend
                  wrapperStyle={{ fontSize: 11, paddingTop: 8 }}
                  formatter={(value) => <span style={{ color: '#c8c5b0' }}>{value}</span>}
                />
                {trackedDevices.map((dev, i) => (
                  <Line
                    key={dev}
                    type="monotone"
                    dataKey={dev}
                    stroke={DEVICE_COLORS[i % DEVICE_COLORS.length]}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4 }}
                    connectNulls
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Status Distribution Donut — 1/3 width */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 flex flex-col">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Status Distribution</h3>
          {donutTotal === 0 ? (
            <p className="text-xs text-on-surface-variant text-center my-auto py-8">No data yet</p>
          ) : (
            <div className="flex flex-col items-center justify-center flex-1">
              <ResponsiveContainer width="100%" height={180}>
                <PieChart>
                  <Pie
                    data={donutData}
                    cx="50%"
                    cy="50%"
                    innerRadius={54}
                    outerRadius={78}
                    paddingAngle={3}
                    dataKey="value"
                    labelLine={false}
                  >
                    {donutData.map((entry) => (
                      <Cell key={entry.name} fill={entry.color} stroke="transparent" />
                    ))}
                    {/* SVG centre label via custom component trick */}
                    <DonutCenter cx={0} cy={0} total={donutTotal} />
                  </Pie>
                  <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown, name: unknown) => [Number(v ?? 0), String(name)]} />
                </PieChart>
              </ResponsiveContainer>
              {/* Legend */}
              <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
                {donutData.map((d) => (
                  <div key={d.name} className="flex items-center gap-1.5 text-xs">
                    <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ background: d.color }} />
                    <span className="text-on-surface-variant">{d.name}</span>
                    <span className="font-bold text-on-surface">{d.value}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Charts Row 2: Resource Load */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6">
        <div 
          className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 hover:border-primary/50 hover:shadow-[0_0_15px_rgba(217,253,58,0.1)] transition-all cursor-pointer group"
          onClick={() => setShowResourceModal(true)}
        >
          <div className="flex justify-between items-center mb-3">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest group-hover:text-primary transition-colors">Resource Load</h3>
            <span className="material-symbols-outlined text-on-surface-variant group-hover:text-primary text-sm transition-colors">open_in_full</span>
          </div>
          <div className="space-y-4 mt-6">
            <ResourceBar label="CPU" value={systemInfo.cpu} color="#d9fd3a" />
            <ResourceBar label="Memory" value={systemInfo.memory} color="#cbee29" />
            <ResourceBar label="Error Rate" value={systemInfo.errorRate} color="#ff7351" />
          </div>
        </div>

        {/* Avg Response by Status */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Avg Response by Status</h3>
          <div className="space-y-3 mt-2">
            {(['up', 'warning', 'down'] as const).map((s) => {
              const statusMetrics = metrics.filter((m) => {
                if (s === 'up') return m.status === 'up' || m.status === 'ok';
                if (s === 'warning') return m.status === 'warning' || m.status === 'degraded';
                return m.status === 'down';
              });
              const avg = statusMetrics.length
                ? Math.round(statusMetrics.reduce((acc, m) => acc + (m.response_time || 0), 0) / statusMetrics.length)
                : 0;
              const label = s === 'up' ? 'Healthy' : s === 'warning' ? 'Warning' : 'Down';
              const color = STATUS_COLORS[s];
              const barMax = 2000; // ms reference
              const barWidth = Math.min(100, (avg / barMax) * 100);
              return (
                <div key={s}>
                  <div className="flex justify-between text-xs mb-1">
                    <span style={{ color }}>{label} ({statusMetrics.length} device{statusMetrics.length !== 1 ? 's' : ''})</span>
                    <span className="text-on-surface-variant">{avg}ms</span>
                  </div>
                  <div className="h-2 bg-surface-container-highest rounded">
                    <div className="h-2 rounded transition-all duration-500" style={{ width: `${barWidth}%`, background: color }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Bottom Row: Metrics + Alerts */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Latest Metrics */}
        <div className="bg-surface-container-high rounded-xl p-6 border border-outline-variant/20 flex flex-col shadow-lg">
          <div className="flex items-center gap-2 mb-6">
            <span className="material-symbols-outlined text-primary text-xl">speed</span>
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest text-on-surface">Latest Metrics</h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
                  <th className="pb-3 font-medium">Device</th>
                  <th className="pb-3 font-medium">Protocol</th>
                  <th className="pb-3 font-medium">Status</th>
                  <th className="pb-3 font-medium text-right">Response</th>
                  <th className="pb-3 font-medium text-right">Time</th>
                </tr>
              </thead>
              <tbody className="text-sm">
                {metrics.slice(0, 15).map((m, i) => {
                  const isDown = m.status === 'down';
                  const isWarn = m.status === 'warning' || m.status === 'degraded';
                  const sc = isDown ? 'text-error bg-error/10 border-error/20' : isWarn ? 'text-amber-400 bg-amber-400/10 border-amber-400/20' : 'text-primary bg-primary/10 border-primary/20';
                  const statusIcon = isDown ? 'cancel' : isWarn ? 'warning' : 'check_circle';
                  
                  return (
                    <tr key={m.id || i} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-colors group">
                      <td className="py-3 font-headline font-semibold text-on-surface group-hover:text-primary transition-colors">{m.device_name}</td>
                      <td className="py-3 text-on-surface-variant text-xs uppercase tracking-wider">
                        <div className="flex items-center gap-1.5">
                          <span className="material-symbols-outlined text-[14px] opacity-70">
                            {m.protocol === 'ping'
                              ? 'router'
                              : m.protocol === 'http' || m.protocol === 'https'
                                ? 'public'
                                : m.protocol === 'system'
                                  ? 'memory'
                                  : m.protocol === 'snmp'
                                    ? 'settings_input_antenna'
                                    : 'hub'}
                          </span>
                          {m.protocol || '-'}
                        </div>
                      </td>
                      <td className="py-3">
                        <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border ${sc} text-[10px] font-bold uppercase tracking-widest`}>
                          <span className="material-symbols-outlined text-[14px]">{statusIcon}</span>
                          {m.status}
                        </div>
                      </td>
                      <td className="py-3 text-right font-mono text-on-surface">{m.response_time ?? '-'}ms</td>
                      <td className="py-3 text-right text-xs text-on-surface-variant font-mono">{new Date(m.timestamp || m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {metrics.length === 0 && (
              <div className="flex flex-col items-center justify-center py-12 opacity-50">
                <span className="material-symbols-outlined text-4xl mb-2">monitoring</span>
                <p className="text-xs text-on-surface-variant uppercase tracking-widest">No metrics data yet</p>
              </div>
            )}
          </div>
        </div>

        {/* Active Alerts */}
        <div className="bg-surface-container-high rounded-xl p-6 border border-outline-variant/20 flex flex-col shadow-lg">
          <div className="flex items-center gap-2 mb-6">
            <span className="material-symbols-outlined text-error text-xl animate-pulse">notifications_active</span>
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest text-on-surface">Active Alerts</h3>
            {alerts.length > 0 && (
              <span className="ml-auto bg-error/20 text-error px-2 py-0.5 rounded-full text-[10px] font-bold">{alerts.length}</span>
            )}
          </div>
          <div className="space-y-3">
            {alerts.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 opacity-50 h-full">
                <span className="material-symbols-outlined text-4xl mb-2 text-primary">check_circle</span>
                <p className="text-xs text-on-surface-variant uppercase tracking-widest">All Systems Operational</p>
              </div>
            ) : (
              alerts.slice(0, 8).map((alert) => {
                const isCritical = alert.severity === 'critical';
                const isWarn = alert.severity === 'warning';
                const color = isCritical ? 'text-error' : isWarn ? 'text-amber-400' : 'text-primary';
                const bg = isCritical ? 'bg-error/10 border-error/30' : isWarn ? 'bg-amber-400/10 border-amber-400/30' : 'bg-primary/10 border-primary/30';
                const icon = isCritical ? 'error' : isWarn ? 'warning' : 'info';

                return (
                  <div key={alert.id} className={`flex items-start gap-4 p-4 rounded-xl border ${bg} transition-all hover:brightness-110`}>
                    <span className={`material-symbols-outlined ${color} mt-0.5`}>{icon}</span>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-headline font-bold text-sm text-on-surface">{alert.device_name || `Device ${alert.device_id}`}</span>
                        <span className="text-[10px] font-mono text-on-surface-variant">{new Date(alert.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
                      </div>
                      <p className="text-xs text-on-surface-variant font-body">{alert.message}</p>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>
      
      {showExpandedCharts && (
        <ExpandedChartsModal metrics={historyMetrics} onClose={() => setShowExpandedCharts(false)} />
      )}
      
      {showResourceModal && (
        <ResourceLoadModal systemInfo={systemInfo} onClose={() => setShowResourceModal(false)} />
      )}
    </div>
  );
}
