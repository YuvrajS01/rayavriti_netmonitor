import type { DeviceHistory } from '../../api/types';

export interface HistoryRow {
  t: number;
  [key: string]: number | [number, number] | null;
}

/**
 * Merge per-device bucket series into one row per bucket start, keyed by
 * `d<deviceId>` (avg), `d<id>Range` ([min, max]) and `d<id>Down` (failed
 * polls). Buckets a device has no samples for stay null so the line
 * shows a gap instead of inventing a value.
 */
export function mergeHistory(series: readonly DeviceHistory[]): HistoryRow[] {
  const rows = new Map<number, HistoryRow>();
  for (const s of series) {
    for (const p of s.points ?? []) {
      const t = new Date(p.t).getTime();
      if (!Number.isFinite(t)) continue;
      let row = rows.get(t);
      if (!row) {
        row = { t };
        rows.set(t, row);
      }
      const key = `d${s.deviceId}`;
      row[key] = p.samples > 0 ? p.avg : null;
      row[`${key}Range`] = p.min != null && p.max != null && p.samples > 0 ? [p.min, p.max] : null;
      row[`${key}Down`] = p.down;
    }
  }
  return [...rows.values()].sort((a, b) => a.t - b.t);
}

/** Tick label for a bucket start: time for ranges ≤ 2 days, date otherwise. */
export function formatBucket(t: number, spanMs: number): string {
  const d = new Date(t);
  return spanMs > 2 * 86_400_000
    ? d.toLocaleDateString([], { month: 'short', day: 'numeric' })
    : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
