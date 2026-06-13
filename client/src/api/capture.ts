import { v1, wrap } from './http';
import type { CaptureSession, CapturedPacket, NetworkInterface } from './types';

export const getInterfaces = () =>
  v1.get('/capture/interfaces').then((r) => wrap<NetworkInterface[]>(r.data));

export const startCaptureSession = (body: { interface: string; filter?: string }) =>
  v1.post('/capture/start', body).then((r) => wrap<CaptureSession>(r.data));

export const stopCaptureSession = (id: number) =>
  v1.post(`/capture/${id}/stop`).then((r) => wrap<CaptureSession>(r.data));

export const getCaptureSession = (id: number) =>
  v1.get(`/capture/${id}`).then((r) => wrap<CaptureSession>(r.data));

export const getCapturePackets = (sessionId: number, params: Record<string, number> = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { qs.set(k, String(v)); }
  return v1.get(`/capture/${sessionId}/packets?${qs}`).then((r) => wrap<CapturedPacket[]>(r.data));
};

export const getCaptureSessions = () =>
  v1.get('/capture/sessions').then((r) => wrap<CaptureSession[]>(r.data));
