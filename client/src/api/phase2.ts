import { v1, wrap } from './http';

export type Phase2Row = Record<string, unknown>;

export interface Phase2Summary {
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

export async function getPhase2Summary() {
  const res = await v1.get('/phase2/summary');
  return wrap<Phase2Summary>(res.data);
}

export async function listPhase2(path: string) {
  const res = await v1.get(path);
  return wrap<Phase2Row[]>(res.data);
}

export async function createPhase2(path: string, payload: Phase2Row) {
  const res = await v1.post(path, payload);
  return wrap<Phase2Row>(res.data);
}

export async function updatePhase2(path: string, id: number | string, payload: Phase2Row) {
  const res = await v1.put(`${path}/${id}`, payload);
  return wrap<Phase2Row>(res.data);
}

/** Fetch the device dependency tree from /api/v1/topology */
export async function getTopologyTree() {
  const res = await v1.get('/topology');
  return wrap<TopologyNode[]>(res.data);
}

/** Fetch locations as a nested tree with per-location device status counts */
export async function getLocationTree() {
  const res = await v1.get('/locations?format=tree_with_status');
  return wrap<LocationNode[]>(res.data);
}

/** Fetch device IDs assigned to a specific location */
export async function getLocationDevices(locationId: number) {
  const res = await v1.get(`/locations/${locationId}/devices`);
  return wrap<{ deviceIds: number[] }>(res.data);
}
