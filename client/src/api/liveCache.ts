import type { QueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import type { Alert, DashboardStats, Device, DeviceStatusEvent, Metric } from './types';
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

type AlertListKey = readonly ['alerts', 'list', string, number | null];

/**
 * Write a changed alert into every cached alert list without refetching:
 * replace it where its status still matches the list's filter, drop it
 * where it no longer does. Returns true when some list *should* contain the
 * alert but doesn't (so the caller can invalidate it).
 */
export function patchAlertLists(client: QueryClient, alert: Pick<Alert, 'id'> & Partial<Alert>): boolean {
  let missing = false;
  const entries = client.getQueriesData<Wrapped<Alert[]>>({ queryKey: [...queryKeys.alerts.all, 'list'] });
  for (const [key, value] of entries) {
    if (!value?.data) continue;
    const filter = (key as unknown as AlertListKey)[2];
    const index = value.data.findIndex((a) => a.id === alert.id);
    if (index < 0) {
      if (alert.status && (!filter || filter === alert.status)) missing = true;
      continue;
    }
    const merged: Alert = { ...value.data[index]!, ...alert };
    const keep = !filter || filter === merged.status;
    const next = keep ? value.data.map((a, i) => (i === index ? merged : a)) : value.data.filter((_, i) => i !== index);
    client.setQueryData<Wrapped<Alert[]>>(key, { ...value, data: next });
  }
  return missing;
}

/**
 * `alert:updated` carries the whole alert, so lists are patched in place
 * (no flash, no lost selection); counts and aggregates are refetched.
 */
export function applyAlertUpdated(client: QueryClient, alert: Alert) {
  if (alert && typeof alert.id === 'number') {
    const missing = patchAlertLists(client, alert);
    if (missing) void client.invalidateQueries({ queryKey: [...queryKeys.alerts.all, 'list'] });
  }
  void client.invalidateQueries({ queryKey: queryKeys.alerts.counts() });
  void client.invalidateQueries({ queryKey: [...queryKeys.alerts.all, 'grouped'] });
  void client.invalidateQueries({ queryKey: queryKeys.system.stats });
  void client.invalidateQueries({ queryKey: queryKeys.incidents.all });
}
