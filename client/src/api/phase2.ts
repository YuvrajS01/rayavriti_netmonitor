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
