import MockAdapter from 'axios-mock-adapter';
import { QueryClient } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { v1 } from '../http';
import { addAlertNote, bulkAlertAction, getAlertNotes, getAssignableUsers, setAlertAssignee } from '../alerts';
import { getMetricsHistory, historyWindow } from '../history';
import { searchEntities } from '../search';
import { applyAlertUpdated, patchAlertLists } from '../liveCache';
import { queryKeys } from '../queryKeys';
import type { Alert } from '../types';

const alert = (id: number, status: Alert['status'], extra: Partial<Alert> = {}): Alert => ({
  id, deviceId: 1, severity: 'critical', message: `a${id}`, status, createdAt: '2026-01-01T00:00:00Z', ...extra,
});

describe('v5.1 api contracts', () => {
  let api: MockAdapter;
  beforeEach(() => { api = new MockAdapter(v1); });
  afterEach(() => api.restore());

  it('requests bucketed history for at most 20 devices', async () => {
    api.onGet(/\/metrics\/history/).reply((config) => {
      const qs = new URLSearchParams(config.url!.split('?')[1]);
      expect(qs.get('devices')!.split(',')).toHaveLength(20);
      expect(qs.get('bucket')).toBe('5m');
      expect(qs.get('from')).toBe('2026-01-01T00:00:00.000Z');
      return [200, { success: true, data: [{ deviceId: 1, deviceName: 'core', points: [{ t: 'x', avg: 2, min: 1, max: 3, samples: 4, down: 0 }] }] }];
    });
    const res = await getMetricsHistory({ devices: Array.from({ length: 25 }, (_, i) => i + 1), from: '2026-01-01T00:00:00.000Z', to: '2026-01-01T01:00:00.000Z', bucket: '5m' });
    expect(res[0]!.points[0]!.avg).toBe(2);
  });

  it('treats a null history payload as no series', async () => {
    api.onGet(/\/metrics\/history/).reply(200, { success: true, data: null });
    expect(await getMetricsHistory({ devices: [1], from: 'a', to: 'b' })).toEqual([]);
  });

  it('rounds history windows to the minute so query keys are stable', () => {
    const a = historyWindow(3_600_000, Date.parse('2026-01-01T10:00:20Z'));
    const b = historyWindow(3_600_000, Date.parse('2026-01-01T10:00:50Z'));
    expect(a).toEqual(b);
    expect(a.to).toBe('2026-01-01T10:00:00.000Z');
  });

  it('posts bulk actions with an optional note', async () => {
    api.onPost('/alerts/bulk').reply((config) => {
      expect(JSON.parse(config.data as string)).toEqual({ ids: [1, 2], action: 'acknowledge', note: 'on it' });
      return [200, { success: true, data: { updated: [1], skipped: [{ id: 2, reason: 'invalid_state' }] } }];
    });
    const res = await bulkAlertAction([1, 2], 'acknowledge', 'on it');
    expect(res.data.skipped[0]!.reason).toBe('invalid_state');
  });

  it('reads and writes notes, assignee and assignable users', async () => {
    api.onGet('/alerts/7/notes').reply(200, { success: true, data: [{ id: 1, alertId: 7, text: 'x', author: 'ops', createdAt: 'now' }] });
    api.onPost('/alerts/7/notes').reply(200, { success: true, data: { id: 2, alertId: 7, text: 'y', author: 'ops', createdAt: 'now' } });
    api.onPut('/alerts/7/assignee').reply((config) => [200, { success: true, data: { ...alert(7, 'active'), assignedTo: (JSON.parse(config.data as string) as { userId: number }).userId } }]);
    api.onGet('/users/assignable').reply(200, { success: true, data: [{ id: 3, username: 'ops', displayName: 'Ops' }] });
    expect((await getAlertNotes(7)).data).toHaveLength(1);
    expect((await addAlertNote(7, 'y')).data.text).toBe('y');
    expect((await setAlertAssignee(7, 3)).data.assignedTo).toBe(3);
    expect((await getAssignableUsers()).data[0]!.displayName).toBe('Ops');
  });

  it('normalises partial search results', async () => {
    api.onGet(/\/search\?/).reply(200, { success: true, data: { devices: [{ id: 1, name: 'core', ipAddress: '10.0.0.1', status: 'up' }] } });
    const res = await searchEntities('co');
    expect(res.devices).toHaveLength(1);
    expect(res.alerts).toEqual([]);
    expect(res.locations).toEqual([]);
  });
});

describe('alert cache patching', () => {
  it('moves an updated alert out of lists whose status filter no longer matches', () => {
    const client = new QueryClient();
    client.setQueryData(queryKeys.alerts.list('active', 200), { success: true, data: [alert(1, 'active'), alert(2, 'active')] });
    client.setQueryData(queryKeys.alerts.list('acknowledged', 200), { success: true, data: [alert(1, 'acknowledged')] });

    const missing = patchAlertLists(client, { id: 2, status: 'acknowledged', assignedToName: 'ops' });

    expect(client.getQueryData<{ data: Alert[] }>(queryKeys.alerts.list('active', 200))!.data.map((a) => a.id)).toEqual([1]);
    // The acknowledged list doesn't hold #2 yet, so the caller should refetch it.
    expect(missing).toBe(true);
  });

  it('patches in place where the status still matches', () => {
    const client = new QueryClient();
    client.setQueryData(queryKeys.alerts.list('active', 200), { success: true, data: [alert(1, 'active')] });

    applyAlertUpdated(client, alert(1, 'active', { assignedTo: 4, assignedToName: 'Asha' }));

    const row = client.getQueryData<{ data: Alert[] }>(queryKeys.alerts.list('active', 200))!.data[0]!;
    expect(row.assignedToName).toBe('Asha');
  });
});
