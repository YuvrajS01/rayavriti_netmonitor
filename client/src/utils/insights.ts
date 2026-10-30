import type { DeviceHealth } from '../api/types';

/**
 * One entry per device, worst score first.
 *
 * Entries are keyed by device id only. Two devices can legitimately share a
 * name (e.g. "sw-a" in two buildings), so merging by name would hide one of
 * them from the operator.
 */
export function dedupeInsights(items: readonly DeviceHealth[]): DeviceHealth[] {
  const byId = new Map<number, DeviceHealth>();
  for (const item of items) {
    const existing = byId.get(item.deviceId);
    if (!existing || item.score < existing.score) byId.set(item.deviceId, item);
  }
  return [...byId.values()].sort((a, b) => a.score - b.score);
}
