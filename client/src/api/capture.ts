import { v1, wrap, type RequestOptions } from './http';
import type { CaptureSession, CapturedPacket, NetworkInterface } from './types';

export const getInterfaces = ({ signal }: RequestOptions = {}) =>
  v1.get('/capture/interfaces', { signal }).then((r) => wrap<NetworkInterface[]>(r.data));

export const startCaptureSession = (body: { interface: string; filter?: string }, { signal }: RequestOptions = {}) =>
  v1.post('/capture/start', body, { signal }).then((r) => wrap<CaptureSession>(r.data));

export const stopCaptureSession = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/capture/${id}/stop`, undefined, { signal }).then((r) => wrap<CaptureSession>(r.data));

export const getCaptureSession = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/capture/${id}`, { signal }).then((r) => wrap<CaptureSession>(r.data));

export const getCapturePackets = (sessionId: number, params: Record<string, number> = {}, { signal }: RequestOptions = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { qs.set(k, String(v)); }
  return v1.get(`/capture/${sessionId}/packets?${qs}`, { signal }).then((r) => wrap<CapturedPacket[]>(r.data));
};

export const getCaptureSessions = ({ signal }: RequestOptions = {}) =>
  v1.get('/capture/sessions', { signal }).then((r) => wrap<CaptureSession[]>(r.data));
