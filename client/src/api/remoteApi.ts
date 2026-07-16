import { v1, wrap } from './http';

export type RemoteInstance = {
  id: number; name: string; url: string; locationLabel?: string; tags: string[];
  pollIntervalS: number; tlsSkipVerify: boolean; status: 'unknown' | 'online' | 'offline' | 'degraded';
  lastSeenAt?: string; lastError?: string; fingerprint?: string; serviceMode?: 'active' | 'readonly' | 'maintenance';
};

export type RemoteSnapshot = {
  id: number; instanceId: number; timestamp: string; deviceCount: number; deviceUpCount: number;
  deviceDownCount: number; alertCount: number; criticalAlerts: number; healthScore: number; latencyMs: number; version?: string;
};

export type RemoteOverview = { totalInstances: number; online: number; offline: number; degraded: number; alertCount: number };
export type RemoteInput = { name: string; url: string; apiKey: string; locationLabel?: string; tags: string[]; pollIntervalS: number; tlsSkipVerify: boolean };

export const getRemoteOverview = () => v1.get('/remote/overview').then((r) => wrap<RemoteOverview>(r.data));
export const getRemoteInstances = () => v1.get('/remote/instances').then((r) => wrap<RemoteInstance[]>(r.data));
export const getRemoteInstance = (id: number) => v1.get(`/remote/instances/${id}`).then((r) => wrap<{ instance: RemoteInstance; snapshot: RemoteSnapshot | null }>(r.data));
export const createRemoteInstance = (input: RemoteInput) => v1.post('/remote/instances', input).then((r) => wrap<RemoteInstance>(r.data));
export const updateRemoteInstance = (id: number, input: RemoteInput) => v1.put(`/remote/instances/${id}`, input).then((r) => wrap<RemoteInstance>(r.data));
export const deleteRemoteInstance = (id: number) => v1.delete(`/remote/instances/${id}`);
export const testRemoteInstance = (id: number) => v1.post(`/remote/instances/${id}/test`).then((r) => wrap<{ reachable: boolean }>(r.data));
export const setRemoteMode = (id: number, mode: 'active' | 'readonly' | 'maintenance') => v1.post(`/remote/instances/${id}/mode`, { mode }).then((r) => wrap<{ mode: string }>(r.data));
export const getRemoteSnapshots = (id: number) => v1.get(`/remote/instances/${id}/snapshots`).then((r) => wrap<RemoteSnapshot[]>(r.data));
export const getRemoteDevices = (id: number) => v1.get(`/remote/instances/${id}/devices`).then((r) => wrap<Record<string, unknown>[]>(r.data));
export const getRemoteAlerts = (id: number) => v1.get(`/remote/instances/${id}/alerts`).then((r) => wrap<{ alerts?: Record<string, unknown>[] }>(r.data));
