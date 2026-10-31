import { createContext, useContext, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getAlerts, getInsights, getLatestMetrics, getStats, getSystemInfo } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import type { DashboardStats, InsightsResponse, Metric, SystemInfo } from '../../api/types';
import { normalizeStatus } from '../../utils/status';

/*
 * Data for dashboard widgets. Every hook uses the same query keys as the rest
 * of the app, so widgets that need the same data share one request (and the
 * live cache bridge keeps metrics and alerts fresh). A widget that isn't on
 * the dashboard doesn't mount, so its data is never fetched.
 */

const EMPTY_METRICS: Metric[] = [];
const EMPTY_STATS: DashboardStats = { totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 };

export function useLatestMetrics() {
  const q = useQuery({ queryKey: queryKeys.metrics.latest, queryFn: ({ signal }) => getLatestMetrics({ signal }) });
  return { ...q, metrics: q.data?.data ?? EMPTY_METRICS };
}

export function useActiveAlerts() {
  const q = useQuery({ queryKey: queryKeys.alerts.list('active'), queryFn: ({ signal }) => getAlerts('active', undefined, { signal }) });
  return { ...q, alerts: q.data?.data ?? [] };
}

/** Stats are aggregates the socket doesn't carry, so they poll. */
export function useStats() {
  const statsQuery = useQuery({ queryKey: queryKeys.system.stats, queryFn: ({ signal }) => getStats({ signal }), refetchInterval: 30_000 });
  const { metrics, isPending: metricsPending } = useLatestMetrics();
  const stats = useMemo<DashboardStats>(() => {
    const online = metrics.filter((x) => normalizeStatus(x.status) === 'up').length;
    const warning = metrics.filter((x) => normalizeStatus(x.status) === 'warning').length;
    const uptimePercent = metrics.length > 0 ? Math.round((online / metrics.length) * 100) : 100;
    return statsQuery.data ? { ...EMPTY_STATS, ...statsQuery.data.data, warningDevices: warning, uptimePercent } : EMPTY_STATS;
  }, [statsQuery.data, metrics]);
  return { stats, isPending: statsQuery.isPending || metricsPending, isError: statsQuery.isError, refetch: statsQuery.refetch };
}

export function useInsights() {
  const q = useQuery({ queryKey: queryKeys.insights.current(), queryFn: ({ signal }) => getInsights({ signal }), refetchInterval: 30_000 });
  const insights: InsightsResponse | null = q.data?.data ?? null;
  return { ...q, insights };
}

/** NetMonitor host load; the endpoint samples for ~1s, so it renders on its own. */
export function useSystemLoad() {
  const q = useQuery({ queryKey: queryKeys.system.info, queryFn: ({ signal }) => getSystemInfo({ signal }) });
  const { metrics } = useLatestMetrics();
  const hostInfo: SystemInfo | undefined = q.data?.data;
  return useMemo(() => {
    const down = metrics.filter((x) => normalizeStatus(x.status) === 'down').length;
    return {
      cpu: Math.round(hostInfo?.cpu.usage ?? 0),
      memory: Math.round(hostInfo?.memory.percent ?? 0),
      errorRate: metrics.length ? Math.round((down / metrics.length) * 100) : 0,
      raw: hostInfo,
    };
  }, [hostInfo, metrics]);
}

// ── Live samples ────────────────────────────────────────────────────────────

export const HISTORY_LIMIT = 500;
/** Live samples collected by DashboardLiveProvider. */
export const LiveContext = createContext<Metric[]>(EMPTY_METRICS);

/** Newest-first, de-duplicated union of live samples and the latest snapshot. */
export function mergeLiveHistory(live: readonly Metric[], latest: readonly Metric[]): Metric[] {
  const seen = new Set<string>();
  const out: Metric[] = [];
  for (const m of [...live, ...latest]) {
    const key = `${m.deviceId}-${m.timestamp}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(m);
    if (out.length >= HISTORY_LIMIT) break;
  }
  return out;
}

export function useHistoryMetrics(): Metric[] {
  const live = useContext(LiveContext);
  const { metrics } = useLatestMetrics();
  return useMemo(() => mergeLiveHistory(live, metrics), [live, metrics]);
}
