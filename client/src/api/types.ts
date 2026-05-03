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
  availabilityPercent: number;
  avgResponseMs: number;
  count: number;
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
