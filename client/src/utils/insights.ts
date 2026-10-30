import type { DeviceHealth } from '../api/types';

/**
 * One entry per device, worst score first. The insights payload can list a
 * device more than once (e.g. once per grouped issue), which used to render
 * the same device twice on the dashboard.
 */
export function dedupeInsights(items: readonly DeviceHealth[]): DeviceHealth[] {
  const byDevice = new Map<number, DeviceHealth>();
  for (const item of items) {
    const existing = byDevice.get(item.deviceId);
    if (!existing || item.score < existing.score) byDevice.set(item.deviceId, item);
  }
  return [...byDevice.values()].sort((a, b) => a.score - b.score);
}
