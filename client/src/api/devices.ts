import { v1, wrap } from './http';
import type { Device, Metric, PortScanResult, PortScanResponse } from './types';

export const getDevices = () =>
  v1.get('/devices').then((r) => wrap<Device[]>(r.data));

export const addDevice = (device: Record<string, unknown>) =>
  v1.post('/devices', device).then((r) => wrap<Device>(r.data));

export const deleteDevice = (id: number) =>
  v1.delete(`/devices/${id}`).then((r) => r.data);

export const getLatestMetrics = () =>
  v1.get('/metrics/latest').then((r) => wrap<Metric[]>(r.data));

export const getDeviceMetrics = (id: number, limit?: number) =>
  v1.get(`/devices/${id}/metrics${limit ? `?limit=${limit}` : ''}`).then((r) => wrap<Metric[]>(r.data));

export const getDevicePorts = (id: number) =>
  v1.get(`/devices/${id}/ports`).then((r) => wrap<PortScanResult[]>(r.data));

export const scanDevicePorts = (id: number) =>
  v1.post(`/devices/${id}/scan-ports`).then((r) => wrap<PortScanResponse>(r.data));
