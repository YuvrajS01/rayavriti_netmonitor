import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Wall from '../Wall';
import { dataFreshness, parseWallParams, typicalInterval } from '../wall/wallModel';
import { v1 } from '../../api/http';
import { sessionStore } from '../../store/session';
import { renderPage } from '../../test/renderWithProviders';

describe('wall model', () => {
  it('parses rotation and interval with a 10s floor', () => {
    expect(parseWallParams(new URLSearchParams('rotate=overview,alerts,bogus&interval=3'))).toEqual({ views: ['overview', 'alerts'], intervalS: 10 });
    expect(parseWallParams(new URLSearchParams('view=topology'))).toEqual({ views: ['topology'], intervalS: 30 });
    expect(parseWallParams(new URLSearchParams(''))).toEqual({ views: ['overview'], intervalS: 30 });
  });

  it('marks data stale past 2× and 4× the poll interval, and when the socket drops', () => {
    const now = 1_000_000;
    expect(dataFreshness({ lastDataAt: now - 30_000, now, pollIntervalS: 60, connected: true }).status).toBe('up');
    expect(dataFreshness({ lastDataAt: now - 30_000, now, pollIntervalS: 60, connected: false }).status).toBe('warning');
    expect(dataFreshness({ lastDataAt: now - 150_000, now, pollIntervalS: 60, connected: true }).status).toBe('warning');
    expect(dataFreshness({ lastDataAt: now - 300_000, now, pollIntervalS: 60, connected: true })).toMatchObject({ status: 'down', label: '5m 0s old' });
    expect(dataFreshness({ lastDataAt: null, now, pollIntervalS: 60, connected: true }).status).toBe('unknown');
  });

  it('uses the median device interval', () => {
    expect(typicalInterval([30, 60, 300])).toBe(60);
    expect(typicalInterval([])).toBe(60);
  });
});

describe('Wall page', () => {
  let api: MockAdapter;
  beforeEach(() => {
    api = new MockAdapter(v1);
    sessionStore.setAuthenticated({ id: 1, username: 'noc', role: 'admin' });
    localStorage.clear();
    const t = new Date().toISOString();
    api.onGet('/devices').reply(200, { success: true, data: [
      { id: 1, name: 'core-sw', ipAddress: 'a', protocol: 'ping', port: 0, interval: 60, status: 'up', tags: [] },
      { id: 2, name: 'edge-fw', ipAddress: 'b', protocol: 'ping', port: 0, interval: 60, status: 'down', tags: [] },
    ] });
    api.onGet('/metrics/latest').reply(200, { success: true, data: [{ id: 1, deviceId: 1, status: 'up', responseTime: 2, value: null, protocol: 'ping', deviceName: 'core-sw', timestamp: t, createdAt: t }] });
    api.onGet(/\/alerts\?status=active/).reply(200, { success: true, data: [{ id: 9, deviceId: 2, deviceName: 'edge-fw', severity: 'critical', message: 'Host unreachable', status: 'active', createdAt: t }] });
    api.onGet('/locations').reply(200, { success: true, data: [] });
  });
  afterEach(() => {
    api.restore();
    sessionStore.setUnauthenticated();
    delete document.documentElement.dataset.wall;
  });

  it('shows worst-first devices, the alert strip and a reconnect banner without app chrome', async () => {
    renderPage(<Wall />, { url: '/wall' });
    const grid = await screen.findByRole('list', { name: 'Devices, worst first' });
    const tiles = await within(grid).findAllByRole('listitem');
    expect(tiles[0]).toHaveTextContent('edge-fw');
    expect(screen.getByRole('contentinfo', { name: 'Newest active alerts' })).toHaveTextContent('Host unreachable');
    // The idle test socket counts as disconnected.
    expect(screen.getByRole('alert')).toHaveTextContent(/disconnected/);
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull();
    expect(document.documentElement.dataset.wall).toBe('');
    expect(document.documentElement.dataset.theme).toBe('dark');
  });

  it('remembers the chime toggle (off by default)', async () => {
    renderPage(<Wall />, { url: '/wall?rotate=overview,alerts&interval=30' });
    const chime = await screen.findByRole('button', { name: 'Chime on new critical' });
    expect(chime).toHaveAttribute('aria-pressed', 'false');
    await userEvent.click(chime);
    expect(chime).toHaveAttribute('aria-pressed', 'true');
    expect(localStorage.getItem('nm-wall-chime')).toBe('1');
    expect(screen.getByRole('list', { name: 'Rotation' })).toBeInTheDocument();
  });
});
