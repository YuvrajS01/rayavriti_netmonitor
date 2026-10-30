import { act, render, screen } from '@testing-library/react';
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

    expect(await screen.findByText('Network Overview', {}, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getByLabelText('Account menu')).toBeInTheDocument();
  });
});
