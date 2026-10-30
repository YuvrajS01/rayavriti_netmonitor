import { screen, waitFor } from '@testing-library/react';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Dashboard from '../Dashboard';
import { v1 } from '../../api/http';
import { renderPage } from '../../test/renderWithProviders';

const metric = (deviceId: number, responseTime: number) => ({
  id: deviceId, deviceId, status: 'up', responseTime, value: null, protocol: 'ping', deviceName: `d${deviceId}`, timestamp: new Date().toISOString(), createdAt: '',
});

describe('Dashboard response-time chart', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    api.onGet('/system/stats').reply(200, { data: { totalDevices: 8, onlineDevices: 8, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 } });
    api.onGet('/metrics/latest').reply(200, { data: [1, 2, 3, 4, 5, 6, 7, 8].map((id) => metric(id, id * 10)) });
    api.onGet('/insights/current').reply(200, { data: null });
    api.onGet('/system/info').reply(200, { data: { cpu: { usage: 1, cores: 1, model: 'x' }, memory: { used: 1, total: 2, percent: 50 }, disk: { used: 1, total: 2, percent: 50 }, uptime: 1, loadAvg: [0, 0, 0] } });
  });
  afterEach(() => {
    api.restore();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('asks /metrics/history for the six slowest devices over the last hour', async () => {
    api.onGet(/\/metrics\/history/).reply(200, { success: true, data: [] });
    api.onGet(/.*/).reply(200, { data: [] });
    renderPage(<Dashboard />);
    await waitFor(() => expect(api.history.get.some((r) => r.url?.startsWith('/metrics/history'))).toBe(true));
    const url = api.history.get.find((r) => r.url?.startsWith('/metrics/history'))!.url!;
    const qs = new URLSearchParams(url.split('?')[1]);
    expect(qs.get('devices')).toBe('8,7,6,5,4,3');
    expect(qs.get('bucket')).toBe('1m');
    expect(Date.parse(qs.get('to')!) - Date.parse(qs.get('from')!)).toBe(3_600_000);
  });

  it('falls back to live samples on servers without the history endpoint', async () => {
    api.onGet(/\/metrics\/history/).reply(404);
    api.onGet(/.*/).reply(200, { data: [] });
    renderPage(<Dashboard />);
    expect(await screen.findByText(/History is unavailable on this server/)).toBeInTheDocument();
  });
});
