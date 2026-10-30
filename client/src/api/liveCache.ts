import type { QueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import type { DashboardStats, Device, DeviceStatusEvent, Metric } from './types';
import type { BootstrapEvent } from './wsEvents';

/**
 * Pure cache writers for WebSocket events. Kept separate from the React
 * bridge so they can be unit-tested against a bare QueryClient.
 */

type Wrapped<T> = { data: T; success: boolean };

const BOOTSTRAP_FRESH_MS = 5_000;

/** Upsert the latest metric per device, newest first. */
export function applyMetricUpdate(client: QueryClient, metric: Metric) {
  client.setQueryData<Wrapped<Metric[]>>(queryKeys.metrics.latest, (prev) => {
    if (!prev) return prev;
    const list = prev.data || [];
    const existing = list.find((m) => m.deviceId === metric.deviceId);
    const next = !metric.protocol && existing?.protocol ? { ...metric, protocol: existing.protocol } : metric;
    return { ...prev, data: [next, ...list.filter((m) => m.deviceId !== metric.deviceId)] };
  });
}

export function applyDeviceStatus(client: QueryClient, { deviceId, status }: DeviceStatusEvent) {
  client.setQueryData<Wrapped<Device[]>>(queryKeys.devices.list(), (prev) => {
    if (!prev?.data) return prev;
    return { ...prev, data: prev.data.map((d) => (d.id === deviceId ? { ...d, status } : d)) };
  });
}

/**
 * `bootstrap` is sent on every (re)connect. Seed the caches it covers and
 * invalidate everything event-driven, since events may have been missed
 * while disconnected.
 */
export function applyBootstrap(client: QueryClient, payload: BootstrapEvent) {
  if (payload.stats) {
    client.setQueryData<Wrapped<DashboardStats>>(queryKeys.system.stats, (prev) =>
      prev ? { ...prev, data: { ...prev.data, ...payload.stats } } : prev,
    );
  }
  const latest = payload.latestMetrics;
  if (latest) {
    client.setQueryData<Wrapped<Metric[]>>(queryKeys.metrics.latest, (prev) => {
      if (!prev) return prev;
      const byDevice = new Map((prev.data || []).map((m) => [m.deviceId, m]));
      for (const bm of latest) {
        const current = byDevice.get(bm.deviceId);
        if (!current || new Date(bm.timestamp) > new Date(current.timestamp)) byDevice.set(bm.deviceId, bm);
      }
      return { ...prev, data: [...byDevice.values()] };
    });
  }
  // Skip anything fetched moments ago (e.g. the initial page load racing
  // the first connect) to avoid a redundant round-trip.
  const olderThan = Date.now() - BOOTSTRAP_FRESH_MS;
  const predicate = (q: { state: { dataUpdatedAt: number } }) => q.state.dataUpdatedAt < olderThan;
  void client.invalidateQueries({ queryKey: queryKeys.alerts.all, predicate });
  void client.invalidateQueries({ queryKey: queryKeys.devices.list(), predicate });
}

/** Alert lifecycle changes ripple into alert lists, counts, stats and incidents. */
export function applyAlertChange(client: QueryClient) {
  void client.invalidateQueries({ queryKey: queryKeys.alerts.all });
  void client.invalidateQueries({ queryKey: queryKeys.system.stats });
  void client.invalidateQueries({ queryKey: queryKeys.incidents.all });
}
