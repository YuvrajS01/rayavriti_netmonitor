export interface ApiResponse<T> {
  success: boolean;
  data: T;
  error?: { code: string; message: string };
  meta?: { page: number; limit: number; total: number; totalPages: number };
}

export interface User {
  id: number;
  username: string;
  email: string;
  role: string;
  createdAt: string;
}

export interface AuthTokens {
  token: string;
  refreshToken: string;
  user: User;
}

export interface Device {
  id: number;
  name: string;
  ipAddress: string;
  protocol: string;
  port: number;
  interval: number;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  status: 'up' | 'down' | 'warning' | 'unknown';
  tags: string[];
  snmpPort?: number;
  httpPath?: string;
  httpExpectedStatus?: number;
  locationId?: number;
  snmpCommunity?: string | null;
  snmpVersion?: string | null;
  /** SNMPv3 USM credentials. Passphrases come back as "********". */
  snmpV3?: SNMPv3Config | null;
  deviceCategory?: string;
  monitorConfig?: Record<string, string | number | boolean>;
  manufacturer?: string;
  model?: string;
  serialNumber?: string;
  macAddress?: string;
  notes?: string;
  parentDeviceId?: number;
  dependencyPort?: string;
  rackPosition?: string;
  assetTag?: string;
}

/** Payload of the `device:status` WebSocket event (backend scheduler/result_pipeline.go). */
export interface DeviceStatusEvent {
  deviceId: number;
  status: Device['status'];
}

export interface Metric {
  id: number;
  deviceId: number;
  sensorId?: number;
  status: 'up' | 'down' | 'warning' | 'degraded' | 'ok' | 'unknown';
  responseTime: number;
  value: number | null;
  details?: Record<string, unknown>;
  protocol: string;
  deviceName: string;
  timestamp: string;
  createdAt: string;
}

export interface TrafficInterfaceSample {
  index: number;
  name: string;
  inOctets: number;
  outOctets: number;
  speed?: number;
  operStatus?: number;
}

export interface MetricMessagePayload {
  cpu?: { usage?: number; cores?: number };
  memory?: { used?: number; total?: number; percent?: number };
  disk?: { used?: number; total?: number; percent?: number };
  uptime?: number;
  interfaces?: TrafficInterfaceSample[];
}

export interface Alert {
  id: number;
  deviceId: number;
  sensorId?: number;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  status: 'active' | 'acknowledged' | 'resolved';
  deviceName?: string;
  groupId?: string;
  acknowledgedBy?: string;
  acknowledgedAt?: string;
  resolvedBy?: string;
  resolvedAt?: string;
  createdAt: string;
  ruleId?: number;
  /** v5.1: user the alert is assigned to (null = unassigned). */
  assignedTo?: number | null;
  assignedToName?: string | null;
}

/** GET /alerts/{id}/notes item (v5.1). */
export interface AlertNote {
  id: number;
  alertId: number;
  text: string;
  author: string;
  createdAt: string;
}

export type AlertBulkAction = 'acknowledge' | 'resolve';

/** POST /alerts/bulk response (v5.1). */
export interface AlertBulkResult {
  updated: number[];
  skipped: Array<{ id: number; reason: 'not_found' | 'forbidden' | 'invalid_state' }>;
}

/** GET /users/assignable item (v5.1). */
export interface AssignableUser {
  id: number;
  username: string;
  displayName: string;
}

/** One bucket of GET /metrics/history (v5.1). Response times in ms. */
export interface HistoryPoint {
  t: string;
  avg: number | null;
  min: number | null;
  max: number | null;
  samples: number;
  down: number;
}

export interface DeviceHistory {
  deviceId: number;
  deviceName: string;
  points: HistoryPoint[];
}

export type HistoryBucket = '1m' | '5m' | '15m' | '1h' | '1d';

/** GET /search response (v5.1). */
export interface SearchResults {
  devices: Array<{ id: number; name: string; ipAddress: string; status: string; locationName?: string }>;
  locations: Array<{ id: number; name: string; type: string; path?: string }>;
  alerts: Array<{ id: number; message: string; severity: Alert['severity']; deviceName?: string; createdAt: string }>;
}

/** GET /devices/{id}/dependencies (backend campus.DeviceNode lists). */
export interface DeviceDependencyNode {
  deviceId: number;
  name: string;
  host: string;
  status: string;
  category?: string;
  locationId?: number | null;
  parentDeviceId?: number | null;
  dependencyPort?: string;
}

export interface DeviceDependencies {
  ancestors: DeviceDependencyNode[] | null;
  descendants: DeviceDependencyNode[] | null;
}

export interface AlertCounts {
  active: number;
  acknowledged: number;
  resolved: number;
}

export interface DashboardStats {
  totalDevices: number;
  onlineDevices: number;
  offlineDevices: number;
  warningDevices: number;
  uptimePercent: number;
  totalMetrics24h: number;
  activeAlerts: number;
  avgResponseTime: number;
}

export interface DeviceBreakdown {
  deviceId: number;
  deviceName: string;
  protocol: string;
  sampleCount: number;
  downCount: number;
  warnCount: number;
  avgResponse: number;
  minResponse: number;
  maxResponse: number;
  availabilityPercent?: number;
}

export interface ReportTimeseriesPoint {
  bucketTime: string;
  sampleCount: number;
  avgResponse: number;
  downCount: number;
  warnCount: number;
  availabilityPercent?: number;
}

export interface ReportSummary {
  from: string;
  to: string;
  totalSamples: number;
  downSamples: number;
  warningSamples: number;
  availabilityPercent: number;
  averageResponseMs: number;
}

export interface ReportAlert {
  id: number;
  deviceId: number;
  deviceName: string;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  status: string;
  createdAt: string;
  acknowledgedAt?: string;
  resolvedAt?: string;
  comment?: string;
}

export interface SystemInfo {
  cpu: { usage: number; cores: number; model: string };
  memory: { used: number; total: number; percent: number };
  disk: { used: number; total: number; percent: number };
  uptime: number;
  loadAvg: number[];
}

export interface FlowRecord {
  id: number;
  srcIp: string;
  dstIp: string;
  srcPort: number;
  dstPort: number;
  protocol: string;
  bytes: number;
  packets: number;
  timestamp: string;
}

export interface TopTalker {
  ip: string;
  count: number;
}

export interface ProtocolBreakdown {
  protocolName: string;
  protocolNumber: number;
  bytes: number;
  bytesFormatted: string;
  packets: number;
  flows: number;
  percentage: number;
}

export interface FlowStats {
  totalFlows: number;
  totalBytes: number;
  totalPackets: number;
  uniqueSources: number;
  uniqueDestinations: number;
}

export interface FlowTimeseriesPoint {
  bucketTime: string;
  totalBytes: number;
  totalPackets: number;
  flowCount: number;
}

export interface CaptureSession {
  id: number;
  interfaceName: string;
  filter: string | null;
  status: 'running' | 'stopped' | 'error';
  totalPackets: number;
  totalBytes: number;
  protocols: string[];
  startedAt: string;
  stoppedAt: string | null;
  errorMessage: string | null;
}

export interface CapturedPacket {
  id: number;
  sessionId: number;
  timestamp: string;
  srcIp: string;
  dstIp: string;
  srcPort: number;
  dstPort: number;
  protocol: string;
  length: number;
  flags: string;
  payload: string;
}

export interface NetworkInterface {
  name: string;
  addresses: string[];
  flags: string[];
}

export interface PortScanResult {
  id?: number;
  deviceId?: number;
  port: number;
  protocol: string;
  state: 'open' | 'closed';
  service?: string;
  responseTime?: number | null;
  firstSeen?: string;
  lastSeen?: string;
  lastChangedAt?: string;
  scannedAt?: string;
}

export interface PortScanResponse {
  deviceId: number;
  host: string;
  scannedPorts: number;
  openPorts: number;
  results: PortScanResult[];
  changes: Array<{ port: number; from: string; to: string; serviceGuess: string }>;
}

export interface InsightItem {
  deviceId: number;
  deviceName: string;
  score: number;
  status: string;
  type?: string;
  severity?: 'critical' | 'warning' | 'info';
  title?: string;
  message?: string;
}

export interface HealthFactor {
  score: number;
  weight: number;
  penalty: number;
}

export interface HealthFactors {
  availability: HealthFactor;
  latency: HealthFactor;
  alerts: HealthFactor;
  stability: HealthFactor;
  ports: HealthFactor;
}

// DeviceHealthScore mirrors the persisted AI Health score row from the backend
// (/api/v1/health/scores). It carries the headline numeric score used by the
// device modal gauge.
export interface DeviceHealthScore {
  deviceId: number;
  score: number;
  label: 'healthy' | 'watch' | 'risk' | 'critical';
  trend: 'improving' | 'stable' | 'degrading';
  trendDelta: number;
  computedAt: string;
}

export interface DeviceHealth {
  deviceId: number;
  deviceName: string;
  score: number;
  label: 'healthy' | 'watch' | 'risk' | 'critical';
  availabilityPercent: number;
  avgResponseMs: number;
  activeAlerts: number;
  openPorts: number;
  samples: number;
  factors: HealthFactors;
  trend: 'improving' | 'stable' | 'degrading';
  trendDelta: number;
  issues: Array<{
    severity: 'critical' | 'warning' | 'info';
    type: string;
    message: string;
  }>;
}

export interface TopRiskDevice {
  deviceId: number;
  deviceName: string;
  score: number;
  label: string;
  trend: 'improving' | 'stable' | 'degrading';
  trendDelta: number;
  primaryIssue: string;
}

export interface HealthDistribution {
  critical: number;
  risk: number;
  watch: number;
  healthy: number;
}

export interface InsightsResponse {
  generatedAt: string;
  networkScore: number;
  healthDistribution: HealthDistribution;
  topRisks: TopRiskDevice[];
  health: DeviceHealth[];
  insights: InsightItem[];
}

export interface HealthHistoryPoint {
  timestamp: string;
  score: number | null;
  label: string | null;
}

export interface HealthHistoryResponse {
  generatedAt: string;
  hours: number;
  points: HealthHistoryPoint[];
}

export type SNMPv3SecurityLevel = 'noAuthNoPriv' | 'authNoPriv' | 'authPriv';

export interface SNMPv3Config {
  securityName: string;
  securityLevel: SNMPv3SecurityLevel;
  authProtocol?: string;
  authPassphrase?: string;
  privProtocol?: string;
  privPassphrase?: string;
  contextName?: string;
}

/** Latest per-interface throughput from SNMP polls (GET /devices/:id/interfaces). */
export interface InterfaceSample {
  deviceId: number;
  time: string;
  ifIndex: number;
  name: string;
  inBps: number;
  outBps: number;
  /** Percent of line speed; null when the speed is unknown. */
  utilization: number | null;
  /** IF-MIB ifOperStatus: 1 up, 2 down, others unknown. */
  operStatus: number;
  speedBps: number;
}

/** An LLDP neighbour reported by a device (GET /devices/:id/neighbors). */
export interface LLDPNeighbor {
  deviceId: number;
  localPortNum: number;
  localPort: string;
  remoteChassisId: string;
  remoteSysName?: string;
  remotePortId: string;
  remotePortDesc?: string;
  remoteMgmtIp?: string;
  remoteDeviceId?: number;
  remoteDeviceName?: string;
  firstSeen: string;
  lastSeen: string;
}

/** One widget on a dashboard's 12-column grid (see components/dashboard/layout.ts). */
export interface DashboardWidget {
  id: string;
  type: string;
  /** Columns spanned on wide screens, 1–12. */
  w: number;
  /** Height step, 1–4 (a minimum height; rows grow to fit). */
  h: number;
  config?: Record<string, unknown>;
}

export interface DashboardLayout {
  version: 1;
  widgets: DashboardWidget[];
}

export interface Dashboard {
  id: number;
  userId: number;
  name: string;
  layout: DashboardLayout;
  /** Shared dashboards are readable by every user; only the owner edits them. */
  shared: boolean;
  ownerName?: string;
  /** Opens first for the current user. */
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

/** One bucket of an interface's throughput (GET /devices/:id/interfaces/:ifIndex/history). */
export interface InterfaceHistoryPoint {
  t: string;
  inBps: number;
  outBps: number;
  maxInBps: number;
  maxOutBps: number;
  maxUtilization: number | null;
}
