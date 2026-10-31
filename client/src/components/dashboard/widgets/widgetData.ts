import { HISTORY_MAX_DEVICES } from '../../../api/history';
import type { Device, Metric } from '../../../api/types';
import { normalizeStatus } from '../../../utils/status';

/** Helpers shared by dashboard widgets and their settings. */

type Config = Record<string, unknown>;

/** A config value as a string, or the fallback when it isn't one. */
export const str = (value: unknown, fallback: string) => (typeof value === 'string' ? value : fallback);

/** Chart height for a widget height step (1–4). */
export const chartHeight = (h: number) => [140, 240, 360, 480][Math.min(4, Math.max(1, h)) - 1]!;

const CHART_DEVICES = 6;

/** The slowest devices in the latest poll (responding ones first). */
export function pickSlowest(metrics: readonly Metric[], count = CHART_DEVICES): number[] {
  return [...metrics]
    .filter((m) => m.responseTime != null)
    .sort((a, b) => (b.responseTime ?? 0) - (a.responseTime ?? 0))
    .slice(0, count)
    .map((m) => m.deviceId);
}

export type DeviceListStatus = 'all' | 'problems' | 'down';

/** Devices matching a device-list widget's filters, problems first. */
export function filterDevices(
  devices: readonly Device[],
  statusById: ReadonlyMap<number, string>,
  { status = 'all', locationId, tag }: { status?: DeviceListStatus; locationId?: number; tag?: string },
): Device[] {
  const rank: Record<string, number> = { down: 0, warning: 1, unknown: 2, up: 3 };
  const wantedTag = tag?.trim().toLowerCase();
  return devices
    .filter((d) => locationId == null || d.locationId === locationId)
    .filter((d) => !wantedTag || d.tags?.some((t) => t.toLowerCase() === wantedTag))
    .filter((d) => {
      const s = normalizeStatus(statusById.get(d.id) ?? d.status);
      return status === 'all' || (status === 'down' ? s === 'down' : s !== 'up');
    })
    .sort((a, b) => {
      const sa = rank[normalizeStatus(statusById.get(a.id) ?? a.status)] ?? 2;
      const sb = rank[normalizeStatus(statusById.get(b.id) ?? b.status)] ?? 2;
      return sa - sb || a.name.localeCompare(b.name);
    });
}

/** Why a widget's settings can't be applied, or null. */
export function settingsProblem(type: string, config: Config): string | null {
  if (type === 'device.history') {
    const ids = Array.isArray(config.deviceIds) ? config.deviceIds : [];
    if (!ids.length) return 'Choose at least one device.';
    if (ids.length > HISTORY_MAX_DEVICES) return `Choose at most ${HISTORY_MAX_DEVICES} devices.`;
  }
  if (type === 'interface.throughput' && (typeof config.deviceId !== 'number' || typeof config.ifIndex !== 'number')) {
    return 'Choose a device and one of its interfaces.';
  }
  if (type === 'device.list') {
    const limit = Number(config.limit);
    if (!Number.isInteger(limit) || limit < 1 || limit > 50) return 'Show between 1 and 50 devices.';
  }
  return null;
}
