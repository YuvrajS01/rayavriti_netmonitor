import { api, wrap } from './http';
import type { Device, Metric, PortScanResult, PortScanResponse } from './types';

export const getDevices = () =>
  api.get('/devices').then((r) => wrap<Device[]>(r.data));

export const addDevice = (device: Record<string, unknown>) =>
  api.post('/devices', device).then((r) => wrap<Device>(r.data));

export const deleteDevice = (id: number) =>
  api.delete(`/devices/${id}`).then((r) => r.data);

export const getLatestMetrics = () =>
  api.get('/metrics/latest').then((r) => wrap<Metric[]>(r.data));

export const getDeviceMetrics = (id: number, limit?: number) =>
  api.get(`/v1/devices/${id}/metrics${limit ? `?limit=${limit}` : ''}`).then((r) => wrap<Metric[]>(r.data));

export const getDevicePorts = (id: number) =>
  api.get(`/devices/${id}/ports`).then((r) => wrap<PortScanResult[]>(r.data));

export const scanDevicePorts = (id: number) =>
  api.post(`/devices/${id}/scan-ports`).then((r) => wrap<PortScanResponse>(r.data));
