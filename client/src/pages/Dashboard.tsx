import { useState, useEffect, useCallback } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { getStats, getLatestMetrics, getAlerts } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, Metric, Alert } from '../api/types';

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

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({ totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 0, activeAlerts: 0, avgResponseTime: 0 });
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [lastUpdated, setLastUpdated] = useState('Waiting for updates...');
  const [systemInfo, setSystemInfo] = useState({ cpu: 0, memory: 0, errorRate: 0 });

  const loadData = useCallback(async () => {
    try {
      const [statsRes, metricsRes, alertsRes] = await Promise.all([
        getStats(), getLatestMetrics(), getAlerts('active'),
      ]);
      setStats(statsRes.data);
      setMetrics(metricsRes.data || []);
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
    // Check if any system collector returned JSON with real CPU/memory data
    const systemMetric = latest.find((x) => x.protocol === 'system');
    let cpu = Math.min(95, Math.round(avgResp / 6 + warn * 4));
    let memory = Math.min(95, Math.round(avgResp / 7 + down * 8 + 28));
    if (systemMetric?.message) {
      try {
        const info = JSON.parse(systemMetric.message);
        if (info.cpu) cpu = Math.round(info.cpu.usage);
        if (info.memory) memory = Math.round(info.memory.percent);
      } catch { /* not JSON, use estimates */ }
    }
    setSystemInfo({
      cpu,
      memory,
      errorRate: Math.min(100, Math.round((down / total) * 100)),
    });
  };

  useEffect(() => { loadData(); }, [loadData]);

  useSocket({
    onBootstrap: (payload) => {
      const p = payload as { stats: DashboardStats; latestMetrics: Metric[]; alerts: Alert[] };
      if (p.stats) setStats(p.stats);
      if (p.latestMetrics) { setMetrics(p.latestMetrics); computeSystemInfo(p.latestMetrics); }
      if (p.alerts) setAlerts(p.alerts);
    },
    onMetricUpdate: (metric) => {
      setMetrics((prev) => {
        const m = metric as Metric;
        const updated = [m, ...prev.filter((x) => x.device_id !== m.device_id)];
        computeSystemInfo(updated);
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

  const chartData = metrics.slice(0, 20).reverse().map((m, i) => ({
    name: i.toString(),
    response: m.response_time || 0,
    device: m.device_name,
  }));

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

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6">
        {/* Live Response Chart */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Live Response Time</h3>
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={chartData}>
              <defs>
                <linearGradient id="responseGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#d9fd3a" stopOpacity={0.35} />
                  <stop offset="100%" stopColor="#d9fd3a" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="name" hide />
              <YAxis hide />
              <Tooltip
                contentStyle={{ background: '#1a1a13', border: '1px solid #494840', borderRadius: '8px', fontSize: '12px', color: '#f4f1e6' }}
                labelFormatter={() => ''}
                formatter={(value: number, _: string, props: { payload: { device: string } }) => [`${value}ms`, props.payload.device]}
              />
              <Area type="monotone" dataKey="response" stroke="#d9fd3a" fill="url(#responseGrad)" strokeWidth={3} />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        {/* Resource Load */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Resource Load</h3>
          <div className="space-y-4 mt-6">
            <ResourceBar label="CPU" value={systemInfo.cpu} color="#d9fd3a" />
            <ResourceBar label="Memory" value={systemInfo.memory} color="#cbee29" />
            <ResourceBar label="Error Rate" value={systemInfo.errorRate} color="#ff7351" />
          </div>
        </div>
      </div>

      {/* Bottom Row: Metrics + Alerts */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Latest Metrics */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Latest Metrics</h3>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="text-on-surface-variant">
                <tr>
                  <th className="text-left pb-2">Device</th>
                  <th className="text-left pb-2">Protocol</th>
                  <th className="text-left pb-2">Status</th>
                  <th className="text-left pb-2">Response</th>
                  <th className="text-left pb-2">Time</th>
                </tr>
              </thead>
              <tbody>
                {metrics.slice(0, 15).map((m, i) => {
                  const statusColor = m.status === 'down' ? 'text-error' : (m.status === 'warning' || m.status === 'degraded') ? 'text-amber-400' : 'text-primary';
                  return (
                    <tr key={m.id || i} className="border-t border-outline-variant/10">
                      <td className="py-1.5">{m.device_name}</td>
                      <td>{m.protocol || '-'}</td>
                      <td className={statusColor}>{m.status}</td>
                      <td>{m.response_time ?? '-'}ms</td>
                      <td className="text-on-surface-variant">{new Date(m.timestamp || m.created_at).toLocaleTimeString()}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {metrics.length === 0 && <p className="text-xs text-on-surface-variant py-4 text-center">No metrics data yet</p>}
          </div>
        </div>

        {/* Active Alerts */}
        <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Active Alerts</h3>
          <div className="space-y-2">
            {alerts.length === 0 && <p className="text-xs text-on-surface-variant">No active alerts</p>}
            {alerts.slice(0, 8).map((alert) => (
              <div key={alert.id} className="flex items-center justify-between gap-3 p-3 rounded-lg border border-outline-variant/20 bg-surface-container-low">
                <span className="text-xs">
                  <span className={alert.severity === 'critical' ? 'text-error' : alert.severity === 'warning' ? 'text-amber-400' : 'text-primary'}>
                    [{alert.severity.toUpperCase()}]
                  </span>{' '}
                  {alert.device_name || `Device ${alert.device_id}`}: {alert.message}
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
