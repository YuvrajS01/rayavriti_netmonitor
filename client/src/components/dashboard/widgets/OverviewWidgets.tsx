import { Suspense, useMemo, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getMetricsHistory, historyWindow } from '../../../api/history';
import { queryKeys } from '../../../api/queryKeys';
import type { DeviceHistory, Metric } from '../../../api/types';
import { useNow } from '../../../utils/time';
import { lazyWithRetry } from '../../../utils/lazyWithRetry';
import { normalizeStatus } from '../../../utils/status';
import StatCard from '../../ui/StatCard';
import { AiHealthScore } from '../AiHealthScore';
import { SmartInsights } from '../SmartInsights';
import { ResponseTimeChart } from '../ResponseTimeChart';
import { StatusDistribution } from '../StatusDistribution';
import { ResourceLoadChart } from '../ResourceLoadChart';
import { AvgResponseByStatus } from '../AvgResponseByStatus';
import { LatestMetricsTable } from '../LatestMetricsTable';
import { ActiveAlertsList } from '../ActiveAlertsList';
import { StatusHeatmap } from '../StatusHeatmap';
import NetworkTopologyMini from '../NetworkTopologyMini';
import { pickSlowest } from './widgetData';
import { useActiveAlerts, useHistoryMetrics, useInsights, useLatestMetrics, useStats, useSystemLoad } from '../useDashboardData';

/*
 * The overview's widgets. Each reads its own data through the shared hooks;
 * the card markup stays in the original components.
 */

const ExpandedChartsModal = lazyWithRetry(() => import('../../ExpandedChartsModal'));
const ResourceLoadModal = lazyWithRetry(() => import('../../ResourceLoadModal'));

export function TotalDevicesWidget() {
  const { stats } = useStats();
  return <StatCard label="Total devices" value={stats.totalDevices} />;
}

export function OnlineWidget() {
  const { stats } = useStats();
  return <StatCard label="Online" value={stats.onlineDevices} delta={`${stats.onlineDevices}/${stats.totalDevices || 0}`} />;
}

export function UptimeWidget() {
  const { stats } = useStats();
  return <StatCard label="Devices up" value={stats.uptimePercent ?? 100} format="percent" />;
}

export function ActiveAlertsStatWidget() {
  const { stats } = useStats();
  return (
    <StatCard
      label="Active alerts"
      value={stats.activeAlerts}
      color={stats.activeAlerts ? 'text-status-down' : 'text-fg'}
      delta={stats.activeAlerts ? 'Needs attention' : 'Clear'}
    />
  );
}

export function HealthScoreWidget() {
  const { insights } = useInsights();
  const { stats } = useStats();
  const health = insights?.health ?? [];
  const networkHealth = health.length ? Math.round(health.reduce((sum, item) => sum + item.score, 0) / health.length) : stats.uptimePercent ?? 100;
  return <AiHealthScore networkHealth={networkHealth} insights={insights} />;
}

export function InsightsWidget() {
  const { insights } = useInsights();
  return <SmartInsights insights={insights} />;
}

const CHART_RANGE_MS = 3_600_000;


/** Older servers: one point per live sample for the chosen devices. */
function liveSeries(metrics: readonly Metric[], ids: number[]): DeviceHistory[] {
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

export function SlowestResponseWidget() {
  const { metrics } = useLatestMetrics();
  const historyMetrics = useHistoryMetrics();
  const [expanded, setExpanded] = useState(false);
  // Re-pick the slowest devices once a minute (with the window), not on every
  // metric update, so the history query key stays stable.
  const now = useNow();
  const win = historyWindow(CHART_RANGE_MS, now);
  const [pick, setPick] = useState<{ at: string; ids: number[] } | null>(null);
  if (metrics.length && pick?.at !== win.to) setPick({ at: win.to, ids: pickSlowest(metrics) });
  const ids = useMemo(() => pick?.ids ?? [], [pick]);
  const params = { devices: ids, ...win, bucket: '1m' as const };
  const history = useQuery({
    queryKey: queryKeys.metrics.history(params),
    queryFn: ({ signal }) => getMetricsHistory(params, { signal }),
    enabled: ids.length > 0,
    retry: false,
    placeholderData: keepPreviousData,
  });
  const series = useMemo(() => (history.isError ? liveSeries(historyMetrics, ids) : history.data ?? []), [history.isError, history.data, historyMetrics, ids]);
  return (
    <>
      <ResponseTimeChart series={series} source={history.isError ? 'live' : 'history'} loading={ids.length > 0 && history.isPending} onExpand={() => setExpanded(true)} />
      <Suspense fallback={null}>{expanded && <ExpandedChartsModal metrics={historyMetrics} onClose={() => setExpanded(false)} />}</Suspense>
    </>
  );
}

export function StatusDistributionWidget() {
  const { metrics } = useLatestMetrics();
  return <StatusDistribution metrics={metrics} />;
}

export function StatusHeatmapWidget() {
  return <StatusHeatmap metrics={useHistoryMetrics()} />;
}

export function TopologyMiniWidget() {
  return <NetworkTopologyMini />;
}

export function ResourceLoadWidget() {
  const systemInfo = useSystemLoad();
  const [expanded, setExpanded] = useState(false);
  return (
    <>
      <ResourceLoadChart systemInfo={systemInfo} onExpand={() => setExpanded(true)} />
      <Suspense fallback={null}>{expanded && <ResourceLoadModal systemInfo={systemInfo} onClose={() => setExpanded(false)} />}</Suspense>
    </>
  );
}

export function ResponseByStatusWidget() {
  const { metrics } = useLatestMetrics();
  return <AvgResponseByStatus metrics={metrics} />;
}

export function LatestMetricsWidget() {
  const { metrics } = useLatestMetrics();
  return <LatestMetricsTable metrics={metrics} />;
}

export function ActiveAlertsListWidget() {
  const { alerts } = useActiveAlerts();
  return <ActiveAlertsList alerts={alerts} />;
}
