/**
 * Typed WebSocket contract.
 *
 * Mirrors the `EventType` constants in backend/internal/websocket/hub.go and
 * the payloads each producer broadcasts. Keep the two in sync: a payload change
 * on the Go side must be reflected here so handlers stay type-checked.
 */
import type { Alert, DashboardStats, DeviceStatusEvent, Metric } from './types';
import type { RemoteSnapshot } from './remoteApi';

/** `metric:update` — backend scheduler/result_pipeline.go broadcasts a models.Metric. */
export type MetricUpdateEvent = Metric;

/** `alert:triggered` — backend engine/alert.go broadcasts the created models.Alert. */
export type AlertTriggeredEvent = Alert;

/**
 * `alert:resolved` — backend engine/alert.go auto-resolve. Note the
 * snake_case keys: this payload is an ad-hoc map, not a model.
 */
export interface AlertResolvedEvent {
  alert_id: number;
  device_id: number;
  device_name: string;
  resolved_by: string;
}

/** `alert:updated` — reserved by the backend; no producer emits it yet. */
export type AlertUpdatedEvent = Partial<Alert> & { id?: number };

/** `bootstrap` — sent once per connection by cmd/server/main.go bootstrapFn. */
export interface BootstrapEvent {
  stats: Partial<DashboardStats>;
  latestMetrics: Metric[] | null;
  alerts: Alert[] | null;
  user: { id: number; username: string; role: string };
}

/** `flow:update` — reserved by the backend; no producer emits it yet. */
export interface FlowUpdateEvent {
  sample?: Array<{ src: string; dst: string; proto: string; bytes: number }>;
  count?: number;
}

/** One packet inside a `capture:packet` batch (handlers/capture.go). */
export interface CapturePacketItem {
  timestamp: string;
  srcIp: string;
  dstIp: string;
  srcPort: number;
  dstPort: number;
  protocol: string;
  length: number;
  flags: string;
  payload?: string;
}

/** `capture:packet` — batched packets for a running capture session. */
export interface CapturePacketEvent {
  sessionId: number;
  packets: CapturePacketItem[];
  count: number;
}

/** `capture:status` — session lifecycle (handlers/capture.go). */
export interface CaptureStatusEvent {
  sessionId: number;
  status: 'running' | 'stopped' | 'error' | (string & {});
  interface?: string;
  error?: string;
}

/** `ports:scanned` — reserved by the backend; no producer emits it yet. */
export interface PortsScannedEvent {
  deviceId: number;
}

/** `remote:status` — remote/collector.go after each poll of a remote instance. */
export interface RemoteStatusEvent {
  instanceId: number;
  data: { status: 'online' | 'offline'; latencyMs?: number; error?: string };
}

/** `remote:metrics` — remote/collector.go snapshot after a successful poll. */
export interface RemoteMetricsEvent {
  instanceId: number;
  data: RemoteSnapshot;
}

/** Event name → payload. The single source of truth for the client contract. */
export interface WsEventMap {
  'metric:update': MetricUpdateEvent;
  'alert:triggered': AlertTriggeredEvent;
  'alert:updated': AlertUpdatedEvent;
  'alert:resolved': AlertResolvedEvent;
  'device:status': DeviceStatusEvent;
  'flow:update': FlowUpdateEvent;
  'capture:packet': CapturePacketEvent;
  'capture:status': CaptureStatusEvent;
  'ports:scanned': PortsScannedEvent;
  bootstrap: BootstrapEvent;
  'remote:status': RemoteStatusEvent;
  'remote:metrics': RemoteMetricsEvent;
}

export type WsEventName = keyof WsEventMap;

/** Discriminated union of every message the backend can send. */
export type WsMessage = {
  [K in WsEventName]: { type: K; request_id?: string; data: WsEventMap[K] };
}[WsEventName];

export type WsHandler<K extends WsEventName> = (data: WsEventMap[K]) => void;

export const WS_EVENT_NAMES: readonly WsEventName[] = [
  'metric:update',
  'alert:triggered',
  'alert:updated',
  'alert:resolved',
  'device:status',
  'flow:update',
  'capture:packet',
  'capture:status',
  'ports:scanned',
  'bootstrap',
  'remote:status',
  'remote:metrics',
];

const EVENT_NAME_SET: ReadonlySet<string> = new Set(WS_EVENT_NAMES);

export function isWsEventName(value: unknown): value is WsEventName {
  return typeof value === 'string' && EVENT_NAME_SET.has(value);
}

/**
 * Parse a raw frame into a WsMessage. Unknown types and malformed JSON return
 * null. The payload shape itself is trusted (validated by the backend contract).
 */
export function parseWsMessage(raw: unknown): WsMessage | null {
  if (typeof raw !== 'string') return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== 'object') return null;
  const { type } = parsed as { type?: unknown };
  if (!isWsEventName(type)) return null;
  return parsed as WsMessage;
}
