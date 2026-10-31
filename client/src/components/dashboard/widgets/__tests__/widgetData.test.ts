import { describe, expect, it } from 'vitest';
import type { Device, Metric } from '../../../../api/types';
import { filterDevices, pickSlowest, settingsProblem } from '../widgetData';

const device = (id: number, name: string, extra: Partial<Device> = {}): Device =>
  ({ id, name, ipAddress: `10.0.0.${id}`, protocol: 'ping', port: 0, interval: 60, enabled: true, createdAt: '', updatedAt: '', status: 'up', tags: [], ...extra });

describe('device list filtering', () => {
  const devices = [
    device(1, 'core', { locationId: 1, tags: ['Core'] }),
    device(2, 'access-a', { locationId: 2 }),
    device(3, 'access-b', { locationId: 2, tags: ['edge'] }),
    device(4, 'printer', { locationId: 2 }),
  ];
  // The latest poll overrides the stored status.
  const live = new Map<number, string>([[2, 'down'], [3, 'warning']]);

  it('puts problems first, then sorts by name', () => {
    expect(filterDevices(devices, live, {}).map((d) => d.name)).toEqual(['access-a', 'access-b', 'core', 'printer']);
  });

  it('filters by location, tag (case-insensitive) and status', () => {
    expect(filterDevices(devices, live, { locationId: 2 }).map((d) => d.id)).toEqual([2, 3, 4]);
    expect(filterDevices(devices, live, { tag: 'core' }).map((d) => d.id)).toEqual([1]);
    expect(filterDevices(devices, live, { status: 'problems' }).map((d) => d.id)).toEqual([2, 3]);
    expect(filterDevices(devices, live, { status: 'down', locationId: 2 }).map((d) => d.id)).toEqual([2]);
  });
});

describe('widget settings validation', () => {
  it('needs devices for response history, within the limit', () => {
    expect(settingsProblem('device.history', { deviceIds: [] })).toMatch(/at least one/);
    expect(settingsProblem('device.history', { deviceIds: Array.from({ length: 21 }, (_, i) => i) })).toMatch(/at most 20/);
    expect(settingsProblem('device.history', { deviceIds: [1] })).toBeNull();
  });

  it('needs a device and interface for throughput', () => {
    expect(settingsProblem('interface.throughput', { deviceId: 1 })).toMatch(/interface/);
    expect(settingsProblem('interface.throughput', { deviceId: 1, ifIndex: 3 })).toBeNull();
  });

  it('bounds the device list length', () => {
    expect(settingsProblem('device.list', { limit: 0 })).not.toBeNull();
    expect(settingsProblem('device.list', { limit: '' })).not.toBeNull();
    expect(settingsProblem('device.list', { limit: 10 })).toBeNull();
  });
});

it('picks the slowest responding devices', () => {
  const m = (deviceId: number, responseTime: number | null) => ({ deviceId, responseTime }) as Metric;
  expect(pickSlowest([m(1, 5), m(2, null), m(3, 50), m(4, 20)], 2)).toEqual([3, 4]);
});
