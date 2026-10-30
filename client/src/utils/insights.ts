import type { DeviceHealth } from '../api/types';

/**
 * One entry per device, worst score first.
 *
 * The insights payload can name the same device more than once: duplicate
 * device records (e.g. "sw-a" registered twice for 10.9.0.1, once manually
 * and once by discovery) each get a score. The dashboard showed those as
 * two identical cards. Entries are merged by device id and by
 * case-insensitive name, keeping the worst score.
 */
export function dedupeInsights(items: readonly DeviceHealth[]): DeviceHealth[] {
  const byKey = new Map<string, DeviceHealth>();
  for (const item of items) {
    const key = item.deviceName?.trim().toLowerCase() || `id:${item.deviceId}`;
    const existing = byKey.get(key);
    if (!existing || item.score < existing.score) byKey.set(key, item);
  }
  return [...byKey.values()].sort((a, b) => a.score - b.score);
}
