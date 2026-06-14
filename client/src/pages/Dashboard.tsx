import { useState, useEffect, useCallback, useMemo, useRef, memo } from 'react';
import {
  LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  PieChart, Pie, Cell,
} from 'recharts';
import { getStats, getLatestMetrics, getAlerts, getInsights, getSystemInfo } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, Metric, Alert, InsightsResponse, SystemInfo } from '../api/types';
import ExpandedChartsModal from '../components/ExpandedChartsModal';
import ResourceLoadModal from '../components/ResourceLoadModal';

const StatCard = memo(function StatCard({ label, value, color = 'text-primary' }: { label: string; value: string | number; color?: string }) {
  return (
    <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-primary/30 content-visibility-auto">
      <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">{label}</p>
      <p className={`font-headline text-3xl font-bold ${color}`}>{value}</p>
    </div>
  );
});

const ResourceBar = memo(function ResourceBar({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span>{label}</span>
        <span>{value}%</span>
      </div>
      <div className="h-2 bg-surface-container-highest rounded">
        <div className="h-2 rounded transition-[width] duration-500" style={{ width: `${Math.min(100, value)}%`, background: color }} />
      </div>
    </div>
  );
});

const DEVICE_COLORS = ['#d9fd3a', '#ff7351', '#6ee7f7', '#c084fc', '#4ade80', '#fb923c'];

const TOOLTIP_STYLE = {
  background: 'var(--color-surface-container)',
  border: '1px solid var(--color-outline-variant)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-on-surface)',
};

interface MultiLinePoint {
  time: string;
  [deviceName: string]: string | number;
}

function buildMultiLineData(metrics: Metric[]): { data: MultiLinePoint[]; devices: string[] } {
  const byDevice = new Map<string, Metric[]>();
  for (const m of metrics) {
    const key = m.deviceName || `Device ${m.deviceId}`;
    if (!byDevice.has(key)) byDevice.set(key, []);
    byDevice.get(key)!.push(m);
  }

  const devices = Array.from(byDevice.keys()).slice(0, 6);

  const primary = byDevice.get(devices[0]) ?? [];
  const timeSlots = primary
    .slice(0, 20)
    .reverse()
    .map((m) => new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }));

  const data: MultiLinePoint[] = timeSlots.map((time, idx) => {
    const point: MultiLinePoint = { time };
    for (const dev of devices) {
      const devMetrics = byDevice.get(dev) ?? [];
      const reversed = [...devMetrics].slice(0, 20).reverse();
      const m = reversed[idx];
      if (m) point[dev] = m.responseTime ?? 0;
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
  const byDevice = new Map<number, string>();
  for (const m of metrics) byDevice.set(m.deviceId, m.status);

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

function DonutCenter({ cx, cy, total }: { cx: number; cy: number; total: number }) {
  return (
    <text x={cx} y={cy} textAnchor="middle" dominantBaseline="middle" fill="#f4f1e6">
      <tspan x={cx} dy="-0.4em" fontSize="22" fontWeight="bold" fontFamily="'Space Grotesk', sans-serif">{total}</tspan>
      <tspan x={cx} dy="1.4em" fontSize="10" fill="#8a8a78" fontFamily="'Space Grotesk', sans-serif">DEVICES</tspan>
    </text>
  );
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({ totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 });
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [historyMetrics, setHistoryMetrics] = useState<Metric[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState('Waiting for updates...');
  const [systemInfo, setSystemInfo] = useState<{ cpu: number; memory: number; errorRate: number; raw?: SystemInfo }>({ cpu: 0, memory: 0, errorRate: 0 });
  const [insights, setInsights] = useState<InsightsResponse | null>(null);
  const [showExpandedCharts, setShowExpandedCharts] = useState(false);
  const [showResourceModal, setShowResourceModal] = useState(false);

  const lastStatsFetch = useRef(0);
  const lastAlertFetch = useRef(0);
  const THROTTLE_MS = 30_000;

  const computeSystemInfo = (m: Metric[]) => {
    const latest = m.slice(0, 40);
    const total = latest.length || 1;
    const down = latest.filter((x) => x.status === 'down').length;
    const warn = latest.filter((x) => x.status === 'warning' || x.status === 'degraded').length;
    const avgResp = latest.reduce((s, x) => s + (x.responseTime || 0), 0) / total;
    const cpu = Math.min(95, Math.round(avgResp / 6 + warn * 4));
    const memory = Math.min(95, Math.round(avgResp / 7 + down * 8 + 28));
    setSystemInfo((prev) => ({
      ...prev,
      cpu,
      memory,
      errorRate: Math.min(100, Math.round((down / total) * 100)),
    }));
  };

  const loadData = useCallback(async () => {
    try {
      const [statsRes, metricsRes, alertsRes] = await Promise.all([
        getStats(), getLatestMetrics(), getAlerts('active'),
      ]);
      const metricsData = metricsRes.data || [];
      const m = metricsData;
      const online = m.filter((x) => x.status === 'up' || x.status === 'ok').length;
      const warning = m.filter((x) => x.status === 'warning' || x.status === 'degraded').length;
      const uptimePercent = m.length > 0 ? Math.round((online / m.length) * 100) : 100;
      setStats({
        ...statsRes.data,
        warningDevices: warning,
        uptimePercent,
      });
      setMetrics(metricsData);
      setHistoryMetrics(metricsData);
      setAlerts(alertsRes.data || []);
      computeSystemInfo(metricsData);
      setLastUpdated(`Loaded ${new Date().toLocaleTimeString()}`);

      // Fetch insights using already-fetched metrics and alerts (no redundant API calls)
      getInsights({
        metrics: metricsData,
        alerts: alertsRes.data || [],
      }).then((r) => setInsights(r.data)).catch(() => {});
    } catch { /* handled by interceptor */ }
    finally {
      setLoading(false);
    }

    // Fire system info in parallel — it has its own error handling and sleeps 1s on the backend,
    // so it should never block the page render.
    getSystemInfo().then((res) => {
      if (res.data) {
        setSystemInfo((prev) => ({ ...prev, raw: res.data }));
      }
    }).catch(() => {});
  }, []);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { loadData(); }, [loadData]);

  useSocket({
    onBootstrap: (payload) => {
      const p = payload as { stats: DashboardStats; latestMetrics: Metric[]; alerts: Alert[] };
      if (p.stats) setStats((prev) => ({ ...prev, ...p.stats }));
      if (p.latestMetrics) {
        // Merge: update latest per device, don't replace history
        setMetrics((prev) => {
          const merged = [...prev];
          for (const bm of p.latestMetrics) {
            const idx = merged.findIndex((m) => m.deviceId === bm.deviceId);
            if (idx === -1) {
              merged.push(bm);
            } else if (new Date(bm.timestamp) > new Date(merged[idx].timestamp)) {
              merged[idx] = bm;
            }
          }
          computeSystemInfo(merged);
          return merged;
        });
        setHistoryMetrics((prev) => {
          const existing = new Set(prev.map((m) => `${m.deviceId}-${m.timestamp}`));
          const merged = [...prev];
          for (const bm of p.latestMetrics) {
            if (!existing.has(`${bm.deviceId}-${bm.timestamp}`)) {
              merged.push(bm);
            }
          }
          if (merged.length > 500) merged.splice(0, merged.length - 500);
          return merged;
        });
      }
      if (p.alerts) setAlerts(p.alerts);
      setLastUpdated(`Connected ${new Date().toLocaleTimeString()}`);
      getSystemInfo().then((res) => {
        if (res.data) {
          setSystemInfo((prev) => ({ ...prev, raw: res.data }));
        }
      }).catch(() => {});
    },
    onMetricUpdate: (metric) => {
      setMetrics((prev) => {
        const m = metric as unknown as Metric;
        if (!m.protocol) {
          const existing = prev.find((x) => x.deviceId === m.deviceId);
          if (existing?.protocol) m.protocol = existing.protocol;
        }
        const updated = [m, ...prev.filter((x) => x.deviceId !== m.deviceId)];
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
      const now = Date.now();
      if (now - lastStatsFetch.current > THROTTLE_MS) {
        lastStatsFetch.current = now;
        getStats().then((r) => setStats((prev) => ({ ...prev, ...r.data }))).catch(() => {});
        getInsights().then((r) => setInsights(r.data)).catch(() => {});
      }
    },
    onAlertTriggered: () => {
      const now = Date.now();
      if (now - lastAlertFetch.current > THROTTLE_MS) {
        lastAlertFetch.current = now;
        Promise.all([getAlerts('active'), getStats()]).then(([a, s]) => {
          setAlerts(a.data || []);
          setStats(s.data);
        }).catch(() => {});
      }
    },
  });

  const { data: multiLineData, devices: trackedDevices } = useMemo(() => buildMultiLineData(historyMetrics), [historyMetrics]);
  const donutData = useMemo(() => buildDonutData(metrics), [metrics]);
  const donutTotal = useMemo(() => donutData.reduce((s, d) => s + d.value, 0), [donutData]);

  const healthArray = useMemo(() => insights?.health || [], [insights]);
  const networkHealth = useMemo(() => healthArray.length
    ? Math.round(healthArray.reduce((sum, item) => sum + item.score, 0) / healthArray.length)
    : stats.uptimePercent ?? 100, [healthArray, stats.uptimePercent]);
  const weakestDevice = useMemo(() => healthArray.length
    ? [...healthArray].sort((a, b) => a.score - b.score)[0]
    : undefined, [healthArray]);

  return (
    <div>
      {loading ? (
        <div className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
          <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
          <p className="text-xs text-on-surface-variant uppercase tracking-widest">Loading dashboard...</p>
        </div>
      ) : (
      <>
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
        <StatCard label="Uptime" value={`${stats.uptimePercent ?? 100}%`} />
        <StatCard label="Active Alerts" value={stats.activeAlerts} color="text-error" />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <div className={`bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 flex flex-col items-center justify-center ${networkHealth < 40 ? 'glow-critical' : networkHealth < 65 ? 'glow-risk' : networkHealth < 85 ? 'glow-watch' : 'glow-healthy'}`}>
          <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-3">AI Health Score</p>
          <div className="relative inline-flex items-center justify-center" style={{ width: 120, height: 120 }}>
            <svg width={120} height={120} className="transform -rotate-90">
              <circle cx={60} cy={60} r={52} fill="none" stroke="#26261d" strokeWidth={8} />
              <circle
                cx={60} cy={60} r={52}
                fill="none"
                stroke={networkHealth < 55 ? '#ff4444' : networkHealth < 75 ? '#f59e0b' : '#d9fd3a'}
                strokeWidth={8}
                strokeLinecap="round"
                strokeDasharray={2 * Math.PI * 52}
                className="gauge-ring"
                style={{
                  '--gauge-circumference': 2 * Math.PI * 52,
                  '--gauge-offset': 2 * Math.PI * 52 - (networkHealth / 100) * 2 * Math.PI * 52,
                  strokeDashoffset: 2 * Math.PI * 52 - (networkHealth / 100) * 2 * Math.PI * 52,
                  filter: `drop-shadow(0 0 6px ${networkHealth < 55 ? '#ff444440' : networkHealth < 75 ? '#f59e0b40' : '#d9fd3a40'})`,
                } as React.CSSProperties}
              />
            </svg>
            <span className={`absolute font-headline text-3xl font-black ${networkHealth < 55 ? 'text-error' : networkHealth < 75 ? 'text-amber-400' : 'text-primary'}`}>
              {networkHealth}
            </span>
          </div>
          {weakestDevice && (
            <div className="flex items-center gap-1 mt-2">
              <span className={`material-symbols-outlined text-sm ${weakestDevice.trend === 'improving' ? 'text-primary' : weakestDevice.trend === 'degrading' ? 'text-error trend-pulse' : 'text-on-surface-variant'}`}>
                {weakestDevice.trend === 'improving' ? 'trending_up' : weakestDevice.trend === 'degrading' ? 'trending_down' : 'trending_flat'}
              </span>
              <span className="text-[10px] uppercase tracking-widest text-on-surface-variant font-bold">
                {weakestDevice.trend || 'stable'}
              </span>
            </div>
          )}
          <p className="text-[10px] text-on-surface-variant mt-2 text-center">
            {weakestDevice ? `${weakestDevice.deviceName} needs watch` : 'Waiting for telemetry'}
          </p>
        </div>

        <div className="xl:col-span-2 bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
          <div className="flex items-center justify-between gap-4 mb-4">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest">Smart Insights</h3>
            <span className="text-[10px] text-on-surface-variant uppercase tracking-widest">
              {insights?.generatedAt ? new Date(insights.generatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'Pending'}
            </span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {healthArray.slice(0, 4).map((item, idx) => {
              const isCritical = item.score < 40;
              const isWarn = item.score < 70;
              const color = isCritical ? 'text-error' : isWarn ? 'text-amber-400' : 'text-primary';
              const bg = isCritical ? 'bg-error/10 border-error/25' : isWarn ? 'bg-amber-400/10 border-amber-400/25' : 'bg-primary/10 border-primary/25';
              return (
                <div key={`${item.deviceId}-${idx}`} className={`rounded-lg border ${bg} p-3 flex gap-3 min-h-24`}>
                  <span className={`material-symbols-outlined ${color} text-lg mt-0.5`}>{isCritical ? 'error' : isWarn ? 'warning' : 'tips_and_updates'}</span>
                  <div className="min-w-0">
                    <p className={`text-[10px] font-bold uppercase tracking-widest ${color}`}>{item.deviceName}</p>
                    <p className="text-xs text-on-surface-variant mt-1 leading-relaxed">Score: {item.score} — {item.label}</p>
                  </div>
                </div>
              );
            })}
            {healthArray.length === 0 && (
              <div className="md:col-span-2 py-8 text-center text-xs text-on-surface-variant">No anomalies or grouped risks detected</div>
            )}
          </div>
        </div>
      </div>

      {/* Charts Row 1: Multi-device response timeline + Status Distribution */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <div
          className="xl:col-span-2 bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 hover:border-primary/50 hover:shadow-[0_0_15px_rgba(217,253,58,0.1)] transition-[border-color,box-shadow] cursor-pointer group"
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
                    <DonutCenter cx={0} cy={0} total={donutTotal} />
                  </Pie>
                  <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown, name: unknown) => [Number(v ?? 0), String(name)]} />
                </PieChart>
              </ResponsiveContainer>
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
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6 content-visibility-auto">
        <div
          className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 hover:border-primary/50 hover:shadow-[0_0_15px_rgba(217,253,58,0.1)] transition-[border-color,box-shadow] cursor-pointer group"
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
                ? Math.round(statusMetrics.reduce((acc, m) => acc + (m.responseTime || 0), 0) / statusMetrics.length)
                : 0;
              const label = s === 'up' ? 'Healthy' : s === 'warning' ? 'Warning' : 'Down';
              const color = STATUS_COLORS[s];
              const barMax = 2000;
              const barWidth = Math.min(100, (avg / barMax) * 100);
              return (
                <div key={s}>
                  <div className="flex justify-between text-xs mb-1">
                    <span style={{ color }}>{label} ({statusMetrics.length} device{statusMetrics.length !== 1 ? 's' : ''})</span>
                    <span className="text-on-surface-variant">{avg}ms</span>
                  </div>
                  <div className="h-2 bg-surface-container-highest rounded">
                    <div className="h-2 rounded transition-[width] duration-500" style={{ width: `${barWidth}%`, background: color }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {/* Bottom Row: Metrics + Alerts */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 content-visibility-auto">
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
                    <tr key={m.id || i} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-[background-color] group">
                      <td className="py-3 font-headline font-semibold text-on-surface group-hover:text-primary transition-[color]">{m.deviceName}</td>
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
                      <td className="py-3 text-right font-mono text-on-surface">{m.responseTime ?? '-'}ms</td>
                      <td className="py-3 text-right text-xs text-on-surface-variant font-mono">{new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
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
                  <div key={alert.id} className={`flex items-start gap-4 p-4 rounded-xl border ${bg} transition-[filter] hover:brightness-110`}>
                    <span className={`material-symbols-outlined ${color} mt-0.5`}>{icon}</span>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-headline font-bold text-sm text-on-surface">{alert.deviceName || `Device ${alert.deviceId}`}</span>
                        <span className="text-[10px] font-mono text-on-surface-variant">{new Date(alert.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
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
      </>
      )}
    </div>
  );
}
