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
  snmpPort: number;
  httpPath?: string;
  httpExpectedStatus?: number;
  locationId?: number;
  snmpCommunity?: string | null;
  snmpVersion?: string | null;
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
  acknowledgedBy?: string;
  resolvedAt?: string;
  createdAt: string;
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
  collectorType: string;
  srcIp: string;
  dstIp: string;
  srcPort: number;
  dstPort: number;
  protocol: number;
  protocolName: string;
  bytes: number;
  packets: number;
  flowStart: string;
  flowEnd: string;
  exporterIp: string;
  timestamp: string;
}

export interface TopTalker {
  ip: string;
  bytes: number;
  bytesFormatted: string;
  packets: number;
  flows: number;
  percentage: number;
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
  totalBytesFormatted: string;
  totalPackets: number;
  uniqueSources: number;
  uniqueDestinations: number;
  activeCollectors: number;
  collectorTypes: string[];
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
