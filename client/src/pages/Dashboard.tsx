import { useState, useMemo, Suspense } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getStats, getLatestMetrics, getAlerts, getInsights, getSystemInfo } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, Metric, InsightsResponse, SystemInfo } from '../api/types';
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
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { normalizeStatus } from '../utils/status';

const ExpandedChartsModal = lazyWithRetry(() => import('../components/ExpandedChartsModal'));
const ResourceLoadModal = lazyWithRetry(() => import('../components/ResourceLoadModal'));

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

const HISTORY_LIMIT = 500;
const EMPTY_STATS: DashboardStats = { totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 };

const historyKey = (m: Metric) => `${m.deviceId}-${m.timestamp}`;

/** Newest-first, de-duplicated union of live samples and the latest snapshot. */
function mergeHistory(live: Metric[], latest: Metric[]): Metric[] {
  const seen = new Set<string>();
  const out: Metric[] = [];
  for (const m of [...live, ...latest]) {
    const key = historyKey(m);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(m);
    if (out.length >= HISTORY_LIMIT) break;
  }
  return out;
}

export default function Dashboard() {
  const [liveMetrics, setLiveMetrics] = useState<Metric[]>([]);
  const [showExpandedCharts, setShowExpandedCharts] = useState(false);
  const [showResourceModal, setShowResourceModal] = useState(false);

  // Stats and insights are aggregates the socket doesn't carry; poll them
  // (TanStack pauses polling while the tab is hidden).
  const statsQuery = useQuery({
    queryKey: queryKeys.system.stats,
    queryFn: ({ signal }) => getStats({ signal }),
    refetchInterval: 30_000,
  });
  // metrics.latest and alerts are kept live by useLiveQueryBridge.
  const metricsQuery = useQuery({
    queryKey: queryKeys.metrics.latest,
    queryFn: ({ signal }) => getLatestMetrics({ signal }),
  });
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list('active'),
    queryFn: ({ signal }) => getAlerts('active', undefined, { signal }),
  });
  const insightsQuery = useQuery({
    queryKey: queryKeys.insights.current(),
    queryFn: ({ signal }) => getInsights({ signal }),
    refetchInterval: 30_000,
  });
  // Host load sleeps ~1s on the backend; it renders independently.
  const hostInfoQuery = useQuery({
    queryKey: queryKeys.system.info,
    queryFn: ({ signal }) => getSystemInfo({ signal }),
  });

  useSocket({
    onMetricUpdate: (metric) => {
      setLiveMetrics((prev) => [metric, ...prev].slice(0, HISTORY_LIMIT));
    },
  });

  const loading = statsQuery.isPending || metricsQuery.isPending || alertsQuery.isPending;
  const metrics = useMemo(() => metricsQuery.data?.data || [], [metricsQuery.data]);
  const alerts = alertsQuery.data?.data || [];
  const insights: InsightsResponse | null = insightsQuery.data?.data ?? null;
  const hostInfo: SystemInfo | undefined = hostInfoQuery.data?.data;
  const historyMetrics = useMemo(() => mergeHistory(liveMetrics, metrics), [liveMetrics, metrics]);
  const lastUpdated = metricsQuery.dataUpdatedAt
    ? `Updated ${new Date(metricsQuery.dataUpdatedAt).toLocaleTimeString()}`
    : 'Waiting for updates...';

  const stats = useMemo<DashboardStats>(() => {
    const online = metrics.filter((x) => normalizeStatus(x.status) === 'up').length;
    const warning = metrics.filter((x) => normalizeStatus(x.status) === 'warning').length;
    const uptimePercent = metrics.length > 0 ? Math.round((online / metrics.length) * 100) : 100;
    return statsQuery.data
      ? { ...EMPTY_STATS, ...statsQuery.data.data, warningDevices: warning, uptimePercent }
      : EMPTY_STATS;
  }, [statsQuery.data, metrics]);

  const { data: multiLineData, devices: trackedDevices } = useMemo(() => buildMultiLineData(historyMetrics), [historyMetrics]);
  const healthArray = useMemo(() => insights?.health || [], [insights]);
  const networkHealth = useMemo(() => healthArray.length
    ? Math.round(healthArray.reduce((sum, item) => sum + item.score, 0) / healthArray.length)
    : stats.uptimePercent ?? 100, [healthArray, stats.uptimePercent]);
  // Server load comes from the NetMonitor host itself (gopsutil); the
  // "devices down" share is computed from the latest poll of each device.
  const systemInfo = useMemo(() => {
    const down = metrics.filter((x) => normalizeStatus(x.status) === 'down').length;
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
        title="Network overview"
        subtitle="Real-time monitoring dashboard for all connected devices."
        action={
          <span className="text-fg-muted font-mono text-xs">{lastUpdated}</span>
        }
      />

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6" aria-live="polite" aria-label="Device statistics">
        <StatCard label="Total devices" value={stats.totalDevices} />
        <StatCard label="Online" value={stats.onlineDevices} delta={`${stats.onlineDevices}/${stats.totalDevices || 0}`} />
        <StatCard label="Devices up" value={stats.uptimePercent ?? 100} format="percent" />
        <StatCard label="Active alerts" value={stats.activeAlerts} color={stats.activeAlerts ? 'text-status-down' : 'text-fg'} delta={stats.activeAlerts ? 'Needs attention' : 'Clear'} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        <AiHealthScore networkHealth={networkHealth} insights={insights} />
        <SmartInsights insights={insights} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <ResponseTimeChart data={multiLineData} devices={trackedDevices} onExpand={() => setShowExpandedCharts(true)} />
        <StatusDistribution metrics={metrics} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6 content-visibility-auto">
        <StatusHeatmap metrics={historyMetrics} />
        <div className="xl:col-span-2 flex flex-col [&>section]:flex-1">
          <NetworkTopologyMini />
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 mb-6 content-visibility-auto">
        <ResourceLoadChart systemInfo={systemInfo} onExpand={() => setShowResourceModal(true)} />
        <AvgResponseByStatus metrics={metrics} />
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6 content-visibility-auto">
        <LatestMetricsTable metrics={metrics} />
        <ActiveAlertsList alerts={alerts} />
      </div>

      <Suspense fallback={null}>
        {showExpandedCharts && (
          <ExpandedChartsModal metrics={historyMetrics} onClose={() => setShowExpandedCharts(false)} />
        )}

        {showResourceModal && (
          <ResourceLoadModal systemInfo={systemInfo} onClose={() => setShowResourceModal(false)} />
        )}
      </Suspense>
      </>
      )}
    </div>
  );
}
