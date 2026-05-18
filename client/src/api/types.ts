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
  created_at: string;
}

export interface AuthTokens {
  token: string;
  refreshToken: string;
  user: User;
}

export interface Device {
  id: number;
  name: string;
  host: string;
  protocol: string;
  port: number;
  interval_seconds: number;
  enabled: number;
  created_at: string;
  snmp_community?: string | null;
  snmp_version?: string | null;
}

export interface Sensor {
  id: number;
  device_id: number;
  type: string;
  name: string;
  config: string;
  enabled: number;
  created_at: string;
  device_name?: string;
}

export interface Metric {
  id: number;
  device_id: number;
  sensor_id?: number;
  status: 'up' | 'down' | 'warning' | 'degraded' | 'ok' | 'unknown';
  response_time: number;
  value: number | null;
  message: string;
  protocol: string;
  device_name: string;
  timestamp: string;
  created_at: string;
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
  device_id: number;
  sensor_id?: number;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  status: 'active' | 'acknowledged' | 'resolved';
  device_name?: string;
  acknowledged_by?: string;
  resolved_at?: string;
  created_at: string;
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
  activeAlerts: number;
  avgResponseTime: number;
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

export interface TimeseriesPoint {
  bucket: string;
  timestamp?: string;
  availabilityPercent: number;
  avgResponseMs: number;
  count: number;
  sampleCount?: number;
  downCount?: number;
}

export interface DeviceBreakdown {
  deviceId: number;
  deviceName: string;
  protocol: string;
  sampleCount: number;
  downCount: number;
  warnCount: number;
  availabilityPercent: number;
  avgResponseMs: number;
  minResponseMs: number;
  maxResponseMs: number;
}

export interface ReportAlert {
  id: number;
  device_id: number;
  device_name: string;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  status: string;
  created_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
  comment?: string;
}

export interface SystemInfo {
  cpu: { usage: number; cores: number; model: string };
  memory: { used: number; total: number; percent: number };
  disk: { used: number; total: number; percent: number };
  uptime: number;
  loadAvg: number[];
}

// ── Flow Analysis Types ──────────────────────────────────────

export interface FlowRecord {
  id: number;
  collector_type: string;
  src_ip: string;
  dst_ip: string;
  src_port: number;
  dst_port: number;
  protocol: number;
  protocol_name: string;
  bytes: number;
  packets: number;
  flow_start: string;
  flow_end: string;
  exporter_ip: string;
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
  protocol_name: string;
  protocol_number: number;
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
  timestamp: string;
  totalBytes: number;
  totalPackets: number;
  flowCount: number;
}

// ── Packet Capture Types ─────────────────────────────────────

export interface CaptureSession {
  id: number;
  interface_name: string;
  filter: string | null;
  status: 'running' | 'stopped' | 'error';
  packet_count: number;
  bytes_captured: number;
  started_at: string;
  stopped_at: string | null;
  error_message: string | null;
}

export interface CapturedPacket {
  no: number;
  session_id: number;
  src_ip: string;
  dst_ip: string;
  src_port: number;
  dst_port: number;
  protocol: string;
  length: number;
  payload_hex: string;
  info: string;
  timestamp: string;
}

export interface NetworkInterface {
  name: string;
  addresses: string[];
  flags: string[];
}

export interface PortScanResult {
  id?: number;
  device_id?: number;
  port: number;
  status: 'open' | 'closed';
  service_guess?: string;
  serviceGuess?: string;
  response_time?: number | null;
  responseTime?: number | null;
  first_seen?: string;
  last_seen?: string;
  last_changed_at?: string;
  message?: string | null;
}

export interface PortScanResponse {
  deviceId: number;
  host: string;
  scannedPorts: number;
  openPorts: PortScanResult[];
  results: PortScanResult[];
  changes: Array<{ port: number; from: string; to: string; serviceGuess: string }>;
}

export interface InsightItem {
  type: string;
  severity: 'critical' | 'warning' | 'info';
  title: string;
  message: string;
  deviceId?: number;
  timestamp?: string;
}

// ── Health Score Factor Breakdown ─────────────────────────────

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
  responseAnomalies: unknown[];
  alertGroups: unknown[];
  flowAnomalies: unknown[];
  insights: InsightItem[];
}

// ── Health History Timeline ──────────────────────────────────

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
