import { v1, wrap, type RequestOptions } from './http';

export interface DiscoveryJob {
  id: number;
  subnet: string;
  scanType: string;
  status: string;
  locationId: number | null;
  initiatedBy: string;
  totalIpsScanned: number;
  devicesFound: number;
  devicesNew: number;
  devicesKnown: number;
  completedAt: string | null;
  errorMessage: string | null;
}

export interface DiscoveryResult {
  id: number;
  jobId: number;
  ipAddress: string;
  macAddress: string;
  hostname: string;
  manufacturer: string;
  guessedCategory: string;
  guessedOs: string;
  openPorts: string;
  snmpReachable: boolean;
  responseTimeMs: number;
  status: string;
  approvedDeviceId: number | null;
  httpTitle: string;
  sshBanner: string;
  tlsCertCn: string;
  snmpName: string;
  snmpDescription: string;
  snmpSysObjectId: string;
}

export const getDiscoveryJobs = ({ signal }: RequestOptions = {}) =>
  v1.get('/discovery/jobs', { signal }).then((r) => wrap<DiscoveryJob[]>(r.data));

export const getDiscoveryResults = (jobId: number, { signal }: RequestOptions = {}) =>
  v1.get(`/discovery/jobs/${jobId}/results`, { signal }).then((r) => wrap<DiscoveryResult[]>(r.data));

export const startDiscoveryScan = (body: { subnet: string; scan_type: string }, { signal }: RequestOptions = {}) =>
  v1.post('/discovery/scan', body, { signal }).then((r) => wrap<DiscoveryJob>(r.data));

export const approveDiscoveryResult = (resultId: number, { signal }: RequestOptions = {}) =>
  v1.post(`/discovery/results/${resultId}/approve`, undefined, { signal }).then((r) => r.data as unknown);
