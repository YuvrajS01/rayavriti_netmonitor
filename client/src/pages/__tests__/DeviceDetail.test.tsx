import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import DeviceDetail from '../DeviceDetail';
import { v1 } from '../../api/http';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

const core = { id: 7, name: 'core-sw', ipAddress: '10.0.0.7', protocol: 'snmp', port: 161, interval: 60, enabled: true, createdAt: '', updatedAt: '', status: 'up', tags: [], parentDeviceId: 2 };

describe('Device detail page', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 1, username: 'admin', role: 'admin' });
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
    api.onGet('/devices/7').reply(200, { success: true, data: core });
    api.onGet('/metrics/latest').reply(200, { success: true, data: [{ id: 1, deviceId: 7, status: 'up', responseTime: 3, value: null, protocol: 'snmp', deviceName: 'core-sw', timestamp: new Date().toISOString(), createdAt: '' }] });
    api.onGet(/\/health\/scores\/7/).reply(200, { success: true, data: { deviceId: 7, score: 92, label: 'healthy', trend: 'stable', trendDelta: 0, computedAt: '' } });
    api.onGet('/locations').reply(200, { success: true, data: [] });
    api.onGet('/devices').reply(200, { success: true, data: [core, { ...core, id: 2, name: 'dist-sw' }] });
  });
  afterEach(() => {
    api.restore();
    vi.unstubAllGlobals();
    sessionStore.setUnauthenticated();
  });

  it('summarises availability from the history endpoint', async () => {
    api.onGet(/\/metrics\/history/).reply((config) => {
      expect(config.url).toContain('devices=7');
      return [200, { success: true, data: [{ deviceId: 7, deviceName: 'core-sw', points: [
        { t: '2026-01-01T00:00:00Z', avg: 4, min: 2, max: 9, samples: 10, down: 0 },
        { t: '2026-01-01T00:15:00Z', avg: 6, min: 3, max: 12, samples: 10, down: 1 },
      ] }] }];
    });
    renderPage(<DeviceDetail />, { url: '/devices/7', path: '/devices/:id' });

    expect(await screen.findByRole('heading', { level: 1, name: /core-sw/ })).toBeInTheDocument();
    expect(await screen.findByText('95.0%')).toBeInTheDocument();
    expect(screen.getByText('1 of 20')).toBeInTheDocument();
    expect(screen.getByText('5 ms')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toHaveTextContent('core-sw');
  });

  it('falls back to recent polls when the history endpoint is missing', async () => {
    api.onGet(/\/metrics\/history/).reply(404);
    api.onGet(/\/devices\/7\/metrics/).reply(200, { success: true, data: [] });
    renderPage(<DeviceDetail />, { url: '/devices/7', path: '/devices/:id' });
    expect(await screen.findByText(/History is unavailable on this server/)).toBeInTheDocument();
  });

  it('deep-links to a tab and shows dependencies', async () => {
    api.onGet('/devices/7/dependencies').reply(200, { success: true, data: {
      ancestors: [{ deviceId: 2, name: 'dist-sw', host: '10.0.0.2', status: 'up' }],
      descendants: [{ deviceId: 9, name: 'ap-1', host: '10.0.0.9', status: 'down', parentDeviceId: 7 }],
    } });
    renderPage(<DeviceDetail />, { url: '/devices/7?tab=dependencies', path: '/devices/:id' });
    expect(await screen.findByRole('tab', { name: /Dependencies/ })).toHaveAttribute('aria-selected', 'true');
    expect(await screen.findByRole('link', { name: 'dist-sw' })).toHaveAttribute('href', '/devices/2');
    expect(screen.getByRole('link', { name: 'ap-1' })).toHaveAttribute('href', '/devices/9');
  });

  it('saves only changed fields and deletes after typing the name', async () => {
    api.onPut('/devices/7').reply(200, { success: true, data: core });
    api.onDelete('/devices/7').reply(200, { success: true });
    renderPage(<DeviceDetail />, { url: '/devices/7?tab=settings', path: '/devices/:id' });

    const interval = await screen.findByLabelText('Poll interval (seconds)');
    await userEvent.clear(interval);
    await userEvent.type(interval, '30');
    await userEvent.click(screen.getByRole('button', { name: 'Save changes' }));
    expect(await screen.findByText('Device saved')).toBeInTheDocument();
    expect(JSON.parse(api.history.put[0]!.data as string)).toEqual({ interval: 30 });

    await userEvent.click(screen.getByRole('button', { name: 'Delete core-sw…' }));
    const dialog = screen.getByRole('alertdialog');
    const confirm = within(dialog).getByRole('button', { name: 'Delete device' });
    await userEvent.type(within(dialog).getByRole('textbox'), 'core-s');
    expect(confirm).toBeDisabled();
    await userEvent.type(within(dialog).getByRole('textbox'), 'w');
    await userEvent.click(confirm);
    expect(await screen.findByTestId('location')).toHaveTextContent('/devices');
  });

  it('says so when the device does not exist', async () => {
    api.onGet('/devices/8').reply(404, { success: false, error: { code: 'not_found', message: 'device not found' } });
    renderPage(<DeviceDetail />, { url: '/devices/8', path: '/devices/:id' });
    expect(await screen.findByText('Device not found')).toBeInTheDocument();
  });
});
