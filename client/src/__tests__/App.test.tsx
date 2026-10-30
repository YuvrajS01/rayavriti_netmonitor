import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MockAdapter from 'axios-mock-adapter';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from '../App';
import { restoreSession } from '../api/auth';
import { authClient, setAccessToken, v1 } from '../api/http';
import { queryClient } from '../api/queryClient';

class IdleWebSocket {
  static readonly OPEN = 1;
  static readonly CONNECTING = 0;
  readyState = 0;
  onopen = null;
  onmessage = null;
  onclose = null;
  onerror = null;
  close() {}
  send() {}
}

describe('App shell', () => {
  let api: MockAdapter;
  let auth: MockAdapter;

  beforeEach(() => {
    api = new MockAdapter(v1);
    auth = new MockAdapter(authClient);
    vi.stubGlobal('WebSocket', IdleWebSocket);
    vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
    vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    window.history.pushState({}, '', '/');
  });

  afterEach(() => {
    api.restore();
    auth.restore();
    setAccessToken(null);
    queryClient.clear();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('sends a visitor without a session to the login page', async () => {
    auth.onPost('/auth/refresh').reply(401);
    await act(() => restoreSession());

    render(<App />);

    expect(await screen.findByRole('button', { name: /sign in|log in|login/i })).toBeInTheDocument();
  });

  it('offers a retry instead of logging out when the server is unreachable', async () => {
    auth.onPost('/auth/refresh').networkError();
    await act(() => restoreSession());

    render(<App />);

    expect(await screen.findByText('Unable to reach the server')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('restores a session from the refresh cookie and renders the dashboard inside the layout', async () => {
    auth.onPost('/auth/refresh').reply(200, { data: { accessToken: 'tok', user: { id: 1, username: 'admin', role: 'admin' } } });
    api.onGet('/system/info').reply(200, { data: { cpu: { usage: 1, cores: 1, model: 'x' }, memory: { used: 1, total: 2, percent: 50 }, disk: { used: 1, total: 2, percent: 50 }, uptime: 1, loadAvg: [0, 0, 0] } });
    api.onGet('/system/stats').reply(200, { data: { totalDevices: 0, onlineDevices: 0, offlineDevices: 0, warningDevices: 0, uptimePercent: 100, totalMetrics24h: 0, activeAlerts: 0, avgResponseTime: 0 } });
    api.onGet('/alerts/counts').reply(200, { data: { active: 0, acknowledged: 0, resolved: 0 } });
    api.onGet('/insights/current').reply(200, { data: null });
    api.onGet(/.*/).reply(200, { data: [] });
    await act(() => restoreSession());

    render(<App />);

    expect(await screen.findByText('Network overview', {}, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getByLabelText('Account menu')).toBeInTheDocument();

    // Layout landmarks and navigation state are exposed to assistive tech.
    expect(document.getElementById('main-content')).toHaveAttribute('tabindex', '-1');
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    const overview = within(nav).getByRole('link', { name: 'Overview' });
    expect(overview).toHaveAttribute('aria-current', 'page');
    expect(within(nav).getByRole('button', { name: 'Operations' })).toHaveAttribute('aria-expanded', 'true');
    expect(within(nav).getByRole('button', { name: 'Reports' })).toHaveAttribute('aria-expanded', 'false');

    // The account menu (with the theme control) closes on Escape and outside clicks.
    const account = screen.getByRole('button', { name: 'Account menu' });
    await userEvent.click(account);
    expect(screen.getByRole('group', { name: 'Theme' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('group', { name: 'Theme' })).toBeNull();
    expect(account).toHaveFocus();
    await userEvent.click(account);
    await userEvent.click(document.body);
    expect(screen.queryByRole('group', { name: 'Theme' })).toBeNull();
  });
});
