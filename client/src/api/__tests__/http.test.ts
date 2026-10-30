import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MockAdapter from 'axios-mock-adapter';
import {
  authClient, getAccessToken, getFreshAccessToken, setAccessToken, setMaintenanceHandler,
  setSessionExpiredHandler, tokenExpiresAt, v1,
} from '../http';

/** Unsigned JWT with the given expiry (seconds from now). */
function jwt(expiresInSec: number, tag = 'x') {
  const b64 = (o: object) => btoa(JSON.stringify(o)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_');
  return `${b64({ alg: 'none' })}.${b64({ exp: Math.floor(Date.now() / 1000) + expiresInSec, tag })}.sig`;
}

const refreshed = (token: string): [number, unknown] => [200, { data: { accessToken: token, user: { id: 1, username: 'admin', role: 'admin' } } }];

describe('http refresh interceptor', () => {
  let api: MockAdapter;
  let auth: MockAdapter;
  let expired: ReturnType<typeof vi.fn<() => void>>;

  beforeEach(() => {
    api = new MockAdapter(v1);
    auth = new MockAdapter(authClient);
    expired = vi.fn<() => void>();
    setSessionExpiredHandler(expired);
    setAccessToken('old-token');
  });

  afterEach(() => {
    api.restore();
    auth.restore();
    setAccessToken(null);
  });

  const authHeader = (config: { headers?: unknown }) =>
    (config.headers as Record<string, string> | undefined)?.Authorization;

  it('refreshes on 401 and replays the request through v1 with the new token', async () => {
    auth.onPost('/auth/refresh').reply(...refreshed('new-token'));
    api.onGet('/devices').replyOnce(401).onGet('/devices').reply((config) =>
      authHeader(config) === 'Bearer new-token' ? [200, { ok: true }] : [401],
    );

    const res = await v1.get('/devices');

    expect(res.data).toEqual({ ok: true });
    expect(getAccessToken()).toBe('new-token');
    expect(auth.history.post).toHaveLength(1);
  });

  it('shares one refresh between concurrent 401s', async () => {
    auth.onPost('/auth/refresh').reply(async () => {
      await new Promise((r) => setTimeout(r, 10));
      return refreshed('new-token');
    });
    api.onGet(/\/(a|b|c)$/).reply((config) =>
      authHeader(config) === 'Bearer new-token' ? [200, config.url] : [401],
    );

    const results = await Promise.all([v1.get('/a'), v1.get('/b'), v1.get('/c')]);

    expect(results.map((r): unknown => r.data)).toEqual(['/a', '/b', '/c']);
    expect(auth.history.post).toHaveLength(1);
  });

  it('replays without refreshing when the token changed while the request was in flight', async () => {
    api.onGet('/slow').replyOnce(() => {
      // Another request refreshed the token meanwhile.
      setAccessToken('already-new');
      return [401];
    });
    api.onGet('/slow').reply((config) => (authHeader(config) === 'Bearer already-new' ? [200, 'ok'] : [401]));

    const res = await v1.get('/slow');

    expect(res.data).toBe('ok');
    expect(auth.history.post).toHaveLength(0);
  });

  it('retries a request at most once', async () => {
    auth.onPost('/auth/refresh').reply(...refreshed('new-token'));
    api.onGet('/forbidden').reply(401);

    await expect(v1.get('/forbidden')).rejects.toMatchObject({ response: { status: 401 } });
    expect(api.history.get).toHaveLength(2);
    expect(auth.history.post).toHaveLength(1);
  });

  it('ends the session when the refresh token is rejected', async () => {
    auth.onPost('/auth/refresh').reply(401);
    api.onGet('/devices').reply(401);

    await expect(v1.get('/devices')).rejects.toMatchObject({ response: { status: 401 } });
    expect(expired).toHaveBeenCalledTimes(1);
    expect(getAccessToken()).toBeNull();
  });

  it('keeps the session when the refresh fails for a non-auth reason', async () => {
    auth.onPost('/auth/refresh').networkError();
    api.onGet('/devices').reply(401);

    await expect(v1.get('/devices')).rejects.toMatchObject({ response: { status: 401 } });
    expect(expired).not.toHaveBeenCalled();
    expect(getAccessToken()).toBe('old-token');
  });

  it('does not try to refresh for credential endpoints', async () => {
    api.onPost('/auth/login').reply(401, { error: 'invalid credentials' });

    await expect(v1.post('/auth/login', {})).rejects.toMatchObject({ response: { status: 401 } });
    expect(auth.history.post).toHaveLength(0);
  });

  it('hands maintenance responses to the maintenance handler', async () => {
    const onMaintenance = vi.fn();
    setMaintenanceHandler(onMaintenance);
    api.onGet('/devices').reply(503, { error: 'System is under maintenance' });

    await expect(v1.get('/devices')).rejects.toBeTruthy();
    expect(onMaintenance).toHaveBeenCalledTimes(1);
  });
});

describe('getFreshAccessToken', () => {
  let auth: MockAdapter;

  beforeEach(() => {
    auth = new MockAdapter(authClient);
    setSessionExpiredHandler(() => {});
  });

  afterEach(() => {
    auth.restore();
    setAccessToken(null);
  });

  it('returns the current token while it is not about to expire', async () => {
    const token = jwt(600);
    setAccessToken(token);
    await expect(getFreshAccessToken()).resolves.toBe(token);
    expect(auth.history.post).toHaveLength(0);
  });

  it('refreshes a token that expires within the skew window', async () => {
    setAccessToken(jwt(5));
    const next = jwt(900, 'next');
    auth.onPost('/auth/refresh').reply(...refreshed(next));
    await expect(getFreshAccessToken()).resolves.toBe(next);
  });

  it('refreshes when there is no token and returns null if that fails', async () => {
    auth.onPost('/auth/refresh').reply(401);
    await expect(getFreshAccessToken()).resolves.toBeNull();
  });

  it('reads the JWT expiry', () => {
    const token = jwt(60);
    const exp = tokenExpiresAt(token)!;
    expect(exp - Date.now()).toBeGreaterThan(55_000);
    expect(tokenExpiresAt('not-a-jwt')).toBeNull();
  });
});
