import { v1, wrap, type RequestOptions } from './http';

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

export const getRemoteOverview = ({ signal }: RequestOptions = {}) => v1.get('/remote/overview', { signal }).then((r) => wrap<RemoteOverview>(r.data));
export const getRemoteInstances = ({ signal }: RequestOptions = {}) => v1.get('/remote/instances', { signal }).then((r) => wrap<RemoteInstance[]>(r.data));
export const getRemoteInstance = (id: number, { signal }: RequestOptions = {}) => v1.get(`/remote/instances/${id}`, { signal }).then((r) => wrap<{ instance: RemoteInstance; snapshot: RemoteSnapshot | null }>(r.data));
export const createRemoteInstance = (input: RemoteInput, { signal }: RequestOptions = {}) => v1.post('/remote/instances', input, { signal }).then((r) => wrap<RemoteInstance>(r.data));
export const updateRemoteInstance = (id: number, input: RemoteInput, { signal }: RequestOptions = {}) => v1.put(`/remote/instances/${id}`, input, { signal }).then((r) => wrap<RemoteInstance>(r.data));
export const deleteRemoteInstance = (id: number, { signal }: RequestOptions = {}) => v1.delete(`/remote/instances/${id}`, { signal });
export const testRemoteInstance = (id: number, { signal }: RequestOptions = {}) => v1.post(`/remote/instances/${id}/test`, undefined, { signal }).then((r) => wrap<{ reachable: boolean }>(r.data));
export const setRemoteMode = (id: number, mode: 'active' | 'readonly' | 'maintenance', { signal }: RequestOptions = {}) => v1.post(`/remote/instances/${id}/mode`, { mode }, { signal }).then((r) => wrap<{ mode: string }>(r.data));
export const getRemoteSnapshots = (id: number, { signal }: RequestOptions = {}) => v1.get(`/remote/instances/${id}/snapshots`, { signal }).then((r) => wrap<RemoteSnapshot[]>(r.data));
export const getRemoteDevices = (id: number, { signal }: RequestOptions = {}) => v1.get(`/remote/instances/${id}/devices`, { signal }).then((r) => wrap<Record<string, unknown>[]>(r.data));
export const getRemoteAlerts = (id: number, { signal }: RequestOptions = {}) => v1.get(`/remote/instances/${id}/alerts`, { signal }).then((r) => wrap<{ alerts?: Record<string, unknown>[] }>(r.data));
