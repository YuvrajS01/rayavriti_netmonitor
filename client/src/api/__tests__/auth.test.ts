import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import { authClient, getAccessToken, setAccessToken, v1 } from '../http';
import { login, logout, restoreSession } from '../auth';
import { queryClient } from '../queryClient';
import { sessionStore } from '../../store/session';

const user = { id: 1, username: 'admin', role: 'admin', permissions: ['devices.read'] };

describe('session lifecycle', () => {
  let api: MockAdapter;
  let auth: MockAdapter;

  beforeEach(() => {
    api = new MockAdapter(v1);
    auth = new MockAdapter(authClient);
    sessionStore.setUnauthenticated();
    localStorage.clear();
  });

  afterEach(() => {
    api.restore();
    auth.restore();
    setAccessToken(null);
    vi.restoreAllMocks();
  });

  it('restores the session from the refresh cookie and keeps the token in memory only', async () => {
    localStorage.setItem('netmonitor_token', 'stale');
    localStorage.setItem('netmonitor_user', '{"id":1}');
    auth.onPost('/auth/refresh').reply(200, { data: { accessToken: 'tok', user } });

    await restoreSession();

    expect(sessionStore.getState()).toEqual({ status: 'authenticated', user });
    expect(getAccessToken()).toBe('tok');
    expect(localStorage.getItem('netmonitor_token')).toBeNull();
    expect(localStorage.getItem('netmonitor_user')).toBeNull();
  });

  it('is unauthenticated when the refresh cookie is missing or rejected', async () => {
    auth.onPost('/auth/refresh').reply(401);
    await restoreSession();
    expect(sessionStore.getState().status).toBe('unauthenticated');
  });

  it('is unreachable (not logged out) when the server cannot be reached', async () => {
    auth.onPost('/auth/refresh').networkError();
    await restoreSession();
    expect(sessionStore.getState().status).toBe('unreachable');
  });

  it('logs in without persisting the token', async () => {
    api.onPost('/auth/login').reply(200, { data: { accessToken: 'tok', refreshToken: 'r', user } });

    await login('admin', 'pw');

    expect(sessionStore.getState()).toEqual({ status: 'authenticated', user });
    expect(getAccessToken()).toBe('tok');
    expect(localStorage.getItem('netmonitor_token')).toBeNull();
  });

  it('logs out locally, clears cached data and notifies other tabs', async () => {
    const post = vi.spyOn(BroadcastChannel.prototype, 'postMessage');
    sessionStore.setAuthenticated(user);
    setAccessToken('tok');
    queryClient.setQueryData(['devices', 'list'], { data: [] });
    api.onPost('/auth/logout').reply(200, { data: { loggedOut: true } });

    await expect(logout()).resolves.toBe(true);

    expect(sessionStore.getState().status).toBe('unauthenticated');
    expect(getAccessToken()).toBeNull();
    expect(queryClient.getQueryData(['devices', 'list'])).toBeUndefined();
    expect(post).toHaveBeenCalledWith({ type: 'logout' });
  });

  it('still ends the local session when the logout request fails', async () => {
    sessionStore.setAuthenticated(user);
    setAccessToken('tok');
    api.onPost('/auth/logout').networkError();

    await expect(logout()).resolves.toBe(false);
    expect(sessionStore.getState().status).toBe('unauthenticated');
    expect(getAccessToken()).toBeNull();
  });

  it('ends the session when a request fails to refresh', async () => {
    sessionStore.setAuthenticated(user);
    setAccessToken('tok');
    api.onGet('/devices').reply(401);
    auth.onPost('/auth/refresh').reply(401);

    await expect(v1.get('/devices')).rejects.toBeTruthy();
    expect(sessionStore.getState().status).toBe('unauthenticated');
  });
});
