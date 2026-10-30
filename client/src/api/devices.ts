import { v1, wrap, type RequestOptions } from './http';
import type { Device, DeviceDependencies, Metric, PortScanResult, PortScanResponse } from './types';

export const getDevices = ({ signal }: RequestOptions = {}) =>
  v1.get('/devices', { signal }).then((r) => wrap<Device[]>(r.data));

export const addDevice = (device: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/devices', device, { signal }).then((r) => wrap<Device>(r.data));

export const updateDevice = (id: number, patch: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.put(`/devices/${id}`, patch, { signal }).then((r) => wrap<Device>(r.data));

export const deleteDevice = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/devices/${id}`, { signal }).then((r): unknown => r.data);

export const getLatestMetrics = ({ signal }: RequestOptions = {}) =>
  v1.get('/metrics/latest', { signal }).then((r) => wrap<Metric[]>(r.data));

export const getDeviceMetrics = (id: number, limit?: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/metrics${limit ? `?limit=${limit}` : ''}`, { signal }).then((r) => wrap<Metric[]>(r.data));

export const getDevicePorts = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/ports`, { signal }).then((r) => wrap<PortScanResult[]>(r.data));

export const scanDevicePorts = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/devices/${id}/scan-ports`, undefined, { signal }).then((r) => wrap<PortScanResponse>(r.data));

export const getDevice = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}`, { signal }).then((r) => wrap<Device>(r.data));

export const getDeviceDependencies = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/dependencies`, { signal }).then((r) => wrap<DeviceDependencies>(r.data));

export const downloadImportTemplate = ({ signal }: RequestOptions = {}) =>
  v1.get('/import/template', { responseType: 'blob', signal }).then((r) => r.data as Blob);
