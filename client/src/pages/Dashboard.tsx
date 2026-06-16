import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { getStats, getLatestMetrics, getAlerts, getInsights, getSystemInfo } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, Metric, Alert, InsightsResponse, SystemInfo } from '../api/types';
import ExpandedChartsModal from '../components/ExpandedChartsModal';
import ResourceLoadModal from '../components/ResourceLoadModal';
import StatCard from '../components/ui/StatCard';
import SectionHeader from '../components/ui/SectionHeader';
import { DashboardSkeleton } from '../components/dashboard/DashboardSkeleton';
import { AiHealthScore } from '../components/dashboard/AiHealthScore';
import { SmartInsights } from '../components/dashboard/SmartInsights';
import { ResponseTimeChart } from '../components/dashboard/ResponseTimeChart';
import { StatusDistribution } from '../components/dashboard/StatusDistribution';
import { ResourceLoadChart } from '../components/dashboard/ResourceLoadChart';
import { AvgResponseByStatus } from '../components/dashboard/AvgResponseByStatus';
import { LatestMetricsTable } from '../components/dashboard/LatestMetricsTable';
import { ActiveAlertsList } from '../components/dashboard/ActiveAlertsList';

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
      const [statsRes, metricsRes, alertsRes, insightsRes] = await Promise.allSettled([
        getStats(), getLatestMetrics(), getAlerts('active'), getInsights(),
      ]);
      const metricsData = metricsRes.status === 'fulfilled' ? (metricsRes.value.data || []) : [];
      const m = metricsData;
      const online = m.filter((x) => x.status === 'up' || x.status === 'ok').length;
      const warning = m.filter((x) => x.status === 'warning' || x.status === 'degraded').length;
      const uptimePercent = m.length > 0 ? Math.round((online / m.length) * 100) : 100;
      if (statsRes.status === 'fulfilled') {
        setStats({
          ...statsRes.value.data,
          warningDevices: warning,
          uptimePercent,
        });
      }
      setMetrics(metricsData);
      setHistoryMetrics(metricsData);
      setAlerts(alertsRes.status === 'fulfilled' ? (alertsRes.value.data || []) : []);
      setInsights(insightsRes.status === 'fulfilled' ? insightsRes.value.data : null);
      computeSystemInfo(metricsData);
      setLastUpdated(`Loaded ${new Date().toLocaleTimeString()}`);
    } catch { /* handled by interceptor */ }
    finally { setLoading(false); }

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
  const healthArray = useMemo(() => insights?.health || [], [insights]);
  const networkHealth = useMemo(() => healthArray.length
    ? Math.round(healthArray.reduce((sum, item) => sum + item.score, 0) / healthArray.length)
    : stats.uptimePercent ?? 100, [healthArray, stats.uptimePercent]);

  return (
    <div>
      {loading ? (
        <DashboardSkeleton />
      ) : (
      <>
      <SectionHeader
        title="Network Overview"
        subtitle="Real-time surveillance dashboard. All systems monitored and reporting."
        action={
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-primary animate-pulse" />
            <span className="text-primary font-mono text-xs">{lastUpdated}</span>
          </div>
        }
      />

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6 mb-12" aria-live="polite" aria-label="Device statistics">
        <StatCard label="Total Devices" value={stats.totalDevices} />
        <StatCard label="Online" value={stats.onlineDevices} />
        <StatCard label="Uptime" value={`${stats.uptimePercent ?? 100}%`} />
        <StatCard label="Active Alerts" value={stats.activeAlerts} color="text-error" />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <AiHealthScore networkHealth={networkHealth} insights={insights} />
        <SmartInsights insights={insights} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <ResponseTimeChart data={multiLineData} devices={trackedDevices} onExpand={() => setShowExpandedCharts(true)} />
        <StatusDistribution metrics={metrics} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6 content-visibility-auto">
        <ResourceLoadChart systemInfo={systemInfo} onExpand={() => setShowResourceModal(true)} />
        <AvgResponseByStatus metrics={metrics} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 content-visibility-auto">
        <LatestMetricsTable metrics={metrics} />
        <ActiveAlertsList alerts={alerts} />
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
