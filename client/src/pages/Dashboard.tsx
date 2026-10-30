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
import { StatusHeatmap } from '../components/dashboard/StatusHeatmap';
import NetworkTopologyMini from '../components/dashboard/NetworkTopologyMini';

interface MultiLinePoint {
  time: string;
  [deviceName: string]: string | number;
}

// One point per poll, keyed by its real timestamp. Each device only has a
// value at the times it was actually polled; the chart connects the gaps.
// (Aligning devices by array index paired up unrelated polls.)
function buildMultiLineData(metrics: Metric[]): { data: MultiLinePoint[]; devices: string[] } {
  const nameOf = (m: Metric) => m.deviceName || `Device ${m.deviceId}`;
  const devices = Array.from(new Set(metrics.map(nameOf))).slice(0, 6);
  const tracked = new Set(devices);

  const points = metrics
    .filter((m) => tracked.has(nameOf(m)) && m.responseTime != null)
    .map((m) => ({ m, t: new Date(m.timestamp || m.createdAt).getTime() }))
    .filter(({ t }) => Number.isFinite(t))
    .sort((a, b) => a.t - b.t)
    .slice(-60);

  const data: MultiLinePoint[] = points.map(({ m, t }) => ({
    time: new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    [nameOf(m)]: m.responseTime ?? 0,
  }));

  return { data, devices };
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>({ totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 });
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [historyMetrics, setHistoryMetrics] = useState<Metric[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState('Waiting for updates...');
  const [hostInfo, setHostInfo] = useState<SystemInfo | undefined>();
  const [insights, setInsights] = useState<InsightsResponse | null>(null);
  const [showExpandedCharts, setShowExpandedCharts] = useState(false);
  const [showResourceModal, setShowResourceModal] = useState(false);

  const lastStatsFetch = useRef(0);
  const lastAlertFetch = useRef(0);
  const THROTTLE_MS = 30_000;

  const loadData = useCallback(async () => {
    try {
      const [statsRes, metricsRes, alertsRes] = await Promise.allSettled([
        getStats(), getLatestMetrics(), getAlerts('active'),
      ]);
      const metricsData = metricsRes.status === 'fulfilled' ? (metricsRes.value.data || []) : [];
      const m = metricsData;
      const online = m.filter((x: Metric) => x.status === 'up' || x.status === 'ok').length;
      const warning = m.filter((x: Metric) => x.status === 'warning' || x.status === 'degraded').length;
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
      setLastUpdated(`Loaded ${new Date().toLocaleTimeString()}`);

      // Fetch insights from server-side health scores
      getInsights().then((r) => setInsights(r.data)).catch(() => {});
    } catch { /* handled by interceptor */ }
    finally {
      setLoading(false);
    }

    // Fire system info in parallel — it has its own error handling and sleeps 1s on the backend,
    // so it should never block the page render.
    getSystemInfo().then((res) => {
      if (res.data) setHostInfo(res.data);
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
        if (res.data) setHostInfo(res.data);
      }).catch(() => {});
    },
    onMetricUpdate: (metric) => {
      setMetrics((prev) => {
        const m = metric as unknown as Metric;
        if (!m.protocol) {
          const existing = prev.find((x) => x.deviceId === m.deviceId);
          if (existing?.protocol) m.protocol = existing.protocol;
        }
        return [m, ...prev.filter((x) => x.deviceId !== m.deviceId)];
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
  // Server load comes from the NetMonitor host itself (gopsutil); the
  // "devices down" share is computed from the latest poll of each device.
  const systemInfo = useMemo(() => {
    const down = metrics.filter((x) => x.status === 'down').length;
    return {
      cpu: Math.round(hostInfo?.cpu.usage ?? 0),
      memory: Math.round(hostInfo?.memory.percent ?? 0),
      errorRate: metrics.length ? Math.round((down / metrics.length) * 100) : 0,
      raw: hostInfo,
    };
  }, [hostInfo, metrics]);

  return (
    <div>
      {loading ? (
        <DashboardSkeleton />
      ) : (
      <>
      <SectionHeader
        title="Network Overview"
        subtitle="Real-time monitoring dashboard for all connected devices."
        action={
          <span className="text-on-surface-variant font-data text-xs">{lastUpdated}</span>
        }
      />

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6" aria-live="polite" aria-label="Device statistics">
        <StatCard label="Total Devices" value={stats.totalDevices} />
        <StatCard label="Online" value={stats.onlineDevices} delta={`${stats.onlineDevices}/${stats.totalDevices || 0}`} />
        <StatCard label="Devices Up" value={stats.uptimePercent ?? 100} format="percent" />
        <StatCard label="Active Alerts" value={stats.activeAlerts} color={stats.activeAlerts ? 'text-error' : 'text-on-surface'} delta={stats.activeAlerts ? 'needs attention' : 'clear'} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <AiHealthScore networkHealth={networkHealth} insights={insights} />
        <SmartInsights insights={insights} />
        <StatusHeatmap metrics={historyMetrics} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <ResponseTimeChart data={multiLineData} devices={trackedDevices} onExpand={() => setShowExpandedCharts(true)} />
        <StatusDistribution metrics={metrics} />
      </div>

      <div className="grid grid-cols-1 gap-6 mb-6 content-visibility-auto">
        <NetworkTopologyMini />
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
