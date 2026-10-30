import { useState, useMemo, Suspense } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getMetricsHistory, historyWindow } from '../api/history';
import { useNow } from '../utils/time';
import { getStats, getLatestMetrics, getAlerts, getInsights, getSystemInfo } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useSocket } from '../hooks/useSocket';
import type { DashboardStats, DeviceHistory, Metric, InsightsResponse, SystemInfo } from '../api/types';
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

const CHART_DEVICES = 6;
const CHART_RANGE_MS = 3_600_000;

/** The slowest devices in the latest poll (responding ones first). */
function pickSlowest(metrics: Metric[]): number[] {
  return [...metrics]
    .filter((m) => m.responseTime != null)
    .sort((a, b) => (b.responseTime ?? 0) - (a.responseTime ?? 0))
    .slice(0, CHART_DEVICES)
    .map((m) => m.deviceId);
}

/** Older servers: one point per live sample for the chosen devices. */
function liveSeries(metrics: Metric[], ids: number[]): DeviceHistory[] {
  return ids.map((id) => {
    const own = metrics.filter((m) => m.deviceId === id && m.responseTime != null);
    return {
      deviceId: id,
      deviceName: own[0]?.deviceName || `Device ${id}`,
      points: own
        .map((m) => ({ t: m.timestamp || m.createdAt, avg: m.responseTime, min: m.responseTime, max: m.responseTime, samples: 1, down: normalizeStatus(m.status) === 'down' ? 1 : 0 }))
        .sort((x, y) => new Date(x.t).getTime() - new Date(y.t).getTime()),
    };
  });
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

  // Re-pick the slowest devices once a minute (with the history window),
  // not on every metric update, so the history query key stays stable.
  const now = useNow();
  const chartWindow = historyWindow(CHART_RANGE_MS, now);
  const [chartPick, setChartPick] = useState<{ at: string; ids: number[] } | null>(null);
  if (metrics.length && chartPick?.at !== chartWindow.to) setChartPick({ at: chartWindow.to, ids: pickSlowest(metrics) });
  const chartIds = useMemo(() => chartPick?.ids ?? [], [chartPick]);
  const historyParams = { devices: chartIds, ...chartWindow, bucket: '1m' as const };
  const historyQuery = useQuery({
    queryKey: queryKeys.metrics.history(historyParams),
    queryFn: ({ signal }) => getMetricsHistory(historyParams, { signal }),
    enabled: chartIds.length > 0,
    retry: false,
    placeholderData: keepPreviousData,
  });
  const chartSource = historyQuery.isError ? 'live' : 'history';
  const chartSeries = useMemo(
    () => (historyQuery.isError ? liveSeries(historyMetrics, chartIds) : historyQuery.data ?? []),
    [historyQuery.isError, historyQuery.data, historyMetrics, chartIds],
  );
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
        <ResponseTimeChart series={chartSeries} source={chartSource} loading={chartIds.length > 0 && historyQuery.isPending} onExpand={() => setShowExpandedCharts(true)} />
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
