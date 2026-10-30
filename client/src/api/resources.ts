import { v1, wrap, type RequestOptions } from './http';

export type ResourceRow = Record<string, unknown>;

export interface ResourceSummary {
  locations: number;
  subnets: number;
  contacts: number;
  incidents: number;
  maintenanceWindows: number;
  statusServices: number;
  discoveryJobs: number;
  ispLinks: number;
  scheduledReports: number;
}

/** Mirrors backend campus.DeviceNode */
export interface TopologyNode {
  deviceId: number;
  name: string;
  host: string;
  status: string;
  category?: string;
  locationId?: number | null;
  parentDeviceId?: number | null;
  dependencyPort?: string;
  children?: TopologyNode[];
}

/** Mirrors backend campus.Location (tree_with_status format) */
export interface LocationNode {
  id: number;
  name: string;
  type: string;
  parent_id: number | null;
  code?: string;
  description?: string;
  address?: string;
  latitude?: number | null;
  longitude?: number | null;
  floor_number?: number | null;
  sort_order?: number;
  enabled?: boolean;
  device_count: number;
  status?: { up: number; down: number; warning: number; maintenance: number; unknown: number };
  children?: LocationNode[];
}

export async function getResourceSummary({ signal }: RequestOptions = {}) {
  const res = await v1.get('/phase2/summary', { signal });
  return wrap<ResourceSummary>(res.data);
}

export async function listResources(path: string, { signal }: RequestOptions = {}) {
  const res = await v1.get(path, { signal });
  return wrap<ResourceRow[]>(res.data);
}

export async function createResource(path: string, payload: ResourceRow, { signal }: RequestOptions = {}) {
  const res = await v1.post(path, payload, { signal });
  return wrap<ResourceRow>(res.data);
}

export async function updateResource(path: string, id: number | string, payload: ResourceRow, { signal }: RequestOptions = {}) {
  const res = await v1.put(`${path}/${id}`, payload, { signal });
  return wrap<ResourceRow>(res.data);
}

export async function deleteResource(path: string, id: number | string, { signal }: RequestOptions = {}) {
  const res = await v1.delete(`${path}/${id}`, { signal });
  return res.data as unknown;
}

/** Fetch the device dependency tree from /api/v1/topology */
export async function getTopologyTree({ signal }: RequestOptions = {}) {
  const res = await v1.get('/topology', { signal });
  return wrap<TopologyNode[]>(res.data);
}

/** Fetch locations as a nested tree with per-location device status counts */
export async function getLocationTree({ signal }: RequestOptions = {}) {
  const res = await v1.get('/locations?format=tree_with_status', { signal });
  return wrap<LocationNode[]>(res.data);
}

/** Fetch device IDs assigned to a specific location */
export async function getLocationDevices(locationId: number, { signal }: RequestOptions = {}) {
  const res = await v1.get(`/locations/${locationId}/devices`, { signal });
  return wrap<{ deviceIds: number[] }>(res.data);
}
