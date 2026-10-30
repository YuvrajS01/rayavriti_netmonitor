import { v1, wrap, type RequestOptions } from './http';
import type { DeviceHistory, HistoryBucket } from './types';

/** The backend accepts at most this many devices per history call. */
export const HISTORY_MAX_DEVICES = 20;

export interface HistoryParams {
  devices: number[];
  from: string;
  to: string;
  bucket?: HistoryBucket;
}

/** Bucketed response-time history per device (GET /metrics/history). */
export const getMetricsHistory = ({ devices, from, to, bucket }: HistoryParams, { signal }: RequestOptions = {}) => {
  const qs = new URLSearchParams({ devices: devices.slice(0, HISTORY_MAX_DEVICES).join(','), from, to });
  if (bucket) qs.set('bucket', bucket);
  return v1.get(`/metrics/history?${qs}`, { signal }).then((r) => {
    const { data } = wrap<DeviceHistory[] | null>(r.data);
    return Array.isArray(data) ? data : [];
  });
};

export interface HistoryRange {
  id: string;
  label: string;
  ms: number;
  bucket: HistoryBucket;
}

export const HISTORY_RANGES: readonly HistoryRange[] = [
  { id: '1h', label: '1h', ms: 3_600_000, bucket: '1m' },
  { id: '6h', label: '6h', ms: 6 * 3_600_000, bucket: '5m' },
  { id: '24h', label: '24h', ms: 24 * 3_600_000, bucket: '15m' },
  { id: '7d', label: '7d', ms: 7 * 24 * 3_600_000, bucket: '1h' },
];

/**
 * A window ending "now", rounded down to the minute so the query key (and
 * so the cache entry) stays stable for a minute rather than changing on
 * every render.
 */
export function historyWindow(rangeMs: number, now = Date.now()): { from: string; to: string } {
  const to = Math.floor(now / 60_000) * 60_000;
  return { from: new Date(to - rangeMs).toISOString(), to: new Date(to).toISOString() };
}
