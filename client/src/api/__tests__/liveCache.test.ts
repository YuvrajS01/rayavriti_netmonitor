import { describe, expect, it } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { applyBootstrap, applyDeviceStatus, applyMetricUpdate } from '../liveCache';
import { queryKeys } from '../queryKeys';
import type { Device, Metric } from '../types';

const metric = (deviceId: number, timestamp: string, extra: Partial<Metric> = {}) =>
  ({ id: 0, deviceId, status: 'up', responseTime: 1, value: null, protocol: 'icmp', deviceName: `d${deviceId}`, timestamp, createdAt: timestamp, ...extra }) as Metric;

const device = (id: number, status: Device['status']) => ({ id, name: `d${id}`, status }) as Device;

describe('liveCache', () => {
  it('upserts metric updates newest-first and keeps a known protocol', () => {
    const client = new QueryClient();
    client.setQueryData(queryKeys.metrics.latest, { data: [metric(1, 't1'), metric(2, 't1')], success: true });

    applyMetricUpdate(client, metric(2, 't2', { protocol: '' }));

    const data = client.getQueryData<{ data: Metric[] }>(queryKeys.metrics.latest)!.data;
    expect(data.map((m) => [m.deviceId, m.timestamp])).toEqual([[2, 't2'], [1, 't1']]);
    expect(data[0]!.protocol).toBe('icmp');
  });

  it('ignores metric updates until the snapshot has loaded', () => {
    const client = new QueryClient();
    applyMetricUpdate(client, metric(1, 't1'));
    expect(client.getQueryData(queryKeys.metrics.latest)).toBeUndefined();
  });

  it('patches device status in the device list', () => {
    const client = new QueryClient();
    client.setQueryData(queryKeys.devices.list(), { data: [device(1, 'up'), device(2, 'up')], success: true });

    applyDeviceStatus(client, { deviceId: 2, status: 'down' });

    const data = client.getQueryData<{ data: Device[] }>(queryKeys.devices.list())!.data;
    expect(data.map((d) => d.status)).toEqual(['up', 'down']);
  });

  it('merges bootstrap metrics, keeping whichever sample is newer', () => {
    const client = new QueryClient();
    client.setQueryData(queryKeys.metrics.latest, {
      data: [metric(1, '2026-01-01T00:00:10Z'), metric(2, '2026-01-01T00:00:00Z')],
      success: true,
    });

    applyBootstrap(client, {
      stats: {},
      alerts: null,
      user: { id: 1, username: 'a', role: 'admin' },
      latestMetrics: [metric(1, '2026-01-01T00:00:05Z'), metric(2, '2026-01-01T00:00:20Z'), metric(3, '2026-01-01T00:00:00Z')],
    });

    const data = client.getQueryData<{ data: Metric[] }>(queryKeys.metrics.latest)!.data;
    const byDevice = Object.fromEntries(data.map((m) => [m.deviceId, m.timestamp]));
    expect(byDevice).toEqual({ 1: '2026-01-01T00:00:10Z', 2: '2026-01-01T00:00:20Z', 3: '2026-01-01T00:00:00Z' });
  });
});
