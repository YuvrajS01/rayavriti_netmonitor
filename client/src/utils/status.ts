/**
 * The single source of truth for status presentation.
 *
 * Every status maps to one colour token, one icon, one label and one shape.
 * Colour is never the only signal: <StatusDot> draws the shape and
 * <StatusPill> adds the label, so the set survives colour-blindness,
 * greyscale printing and a washed-out projector.
 *
 *   up ●  warning ▲  down ■  unknown ?  maintenance ‖  info –
 */

export type Status = 'up' | 'warning' | 'down' | 'unknown' | 'maintenance' | 'info';
export type StatusShape = 'circle' | 'triangle' | 'square' | 'ring' | 'pause' | 'dash';

export interface StatusMeta {
  status: Status;
  /** Colour token name: use as var(--color-<token>) or readToken(token). */
  token: `status-${Status}`;
  label: string;
  icon: string;
  shape: StatusShape;
  /** Literal Tailwind classes (kept literal so the compiler can see them). */
  text: string;
  bg: string;
  border: string;
  /** Single-edge accents for rows/cards that otherwise keep the neutral `border-line`. */
  borderL: string;
  borderT: string;
  /** Tinted chip background (12% wash; contrast-tested with `text`). */
  chip: string;
}

export const STATUS_META: Record<Status, StatusMeta> = {
  up: {
    status: 'up', token: 'status-up', label: 'Up', icon: 'check_circle', shape: 'circle',
    text: 'text-status-up', bg: 'bg-status-up', border: 'border-status-up', borderL: 'border-l-status-up', borderT: 'border-t-status-up', chip: 'bg-status-up/12',
  },
  warning: {
    status: 'warning', token: 'status-warning', label: 'Warning', icon: 'warning', shape: 'triangle',
    text: 'text-status-warning', bg: 'bg-status-warning', border: 'border-status-warning', borderL: 'border-l-status-warning', borderT: 'border-t-status-warning', chip: 'bg-status-warning/12',
  },
  down: {
    status: 'down', token: 'status-down', label: 'Down', icon: 'dangerous', shape: 'square',
    text: 'text-status-down', bg: 'bg-status-down', border: 'border-status-down', borderL: 'border-l-status-down', borderT: 'border-t-status-down', chip: 'bg-status-down/12',
  },
  unknown: {
    status: 'unknown', token: 'status-unknown', label: 'Unknown', icon: 'help', shape: 'ring',
    text: 'text-status-unknown', bg: 'bg-status-unknown', border: 'border-status-unknown', borderL: 'border-l-status-unknown', borderT: 'border-t-status-unknown', chip: 'bg-status-unknown/12',
  },
  maintenance: {
    status: 'maintenance', token: 'status-maintenance', label: 'Maintenance', icon: 'pause_circle', shape: 'pause',
    text: 'text-status-maintenance', bg: 'bg-status-maintenance', border: 'border-status-maintenance', borderL: 'border-l-status-maintenance', borderT: 'border-t-status-maintenance', chip: 'bg-status-maintenance/12',
  },
  info: {
    status: 'info', token: 'status-info', label: 'Info', icon: 'info', shape: 'dash',
    text: 'text-status-info', bg: 'bg-status-info', border: 'border-status-info', borderL: 'border-l-status-info', borderT: 'border-t-status-info', chip: 'bg-status-info/12',
  },
};

const ALIASES: Record<string, Status> = {
  up: 'up', ok: 'up', healthy: 'up', online: 'up', success: 'up', operational: 'up', running: 'up', met: 'up',
  warning: 'warning', warn: 'warning', degraded: 'warning', partial: 'warning', partial_outage: 'warning', slow: 'warning',
  down: 'down', offline: 'down', critical: 'down', error: 'down', failed: 'down', failure: 'down', outage: 'down', major_outage: 'down', breached: 'down',
  maintenance: 'maintenance', paused: 'maintenance', suppressed: 'maintenance', scheduled: 'maintenance',
  info: 'info', informational: 'info', minor: 'info', notice: 'info',
  unknown: 'unknown', pending: 'unknown',
};

/** Maps any backend status/state/severity string onto the six presentation statuses. */
export function normalizeStatus(raw: string | null | undefined): Status {
  if (!raw) return 'unknown';
  return ALIASES[raw.toLowerCase().trim()] ?? 'unknown';
}

export function statusMeta(raw: string | null | undefined): StatusMeta {
  return STATUS_META[normalizeStatus(raw)];
}

/** Device-state label ("Up", "Down"...). */
export function statusLabel(raw: string | null | undefined): string {
  return statusMeta(raw).label;
}

export function statusIcon(raw: string | null | undefined): string {
  return statusMeta(raw).icon;
}

export function statusTextClass(raw: string | null | undefined): string {
  return statusMeta(raw).text;
}

export function statusBgClass(raw: string | null | undefined): string {
  return statusMeta(raw).bg;
}

export function statusBorderClass(raw: string | null | undefined): string {
  return statusMeta(raw).border;
}

/** CSS colour for SVG/recharts (`var(--color-status-down)`). */
export function statusColorVar(raw: string | null | undefined): string {
  return `var(--color-${statusMeta(raw).token})`;
}

// ── Alert severity ─────────────────────────────────────────────

export type Severity = 'critical' | 'warning' | 'info';

export function normalizeSeverity(raw: string | null | undefined): Severity {
  const s = (raw ?? '').toLowerCase();
  if (s === 'critical' || s === 'high' || s === 'major' || s === 'error') return 'critical';
  if (s === 'warning' || s === 'warn' || s === 'medium') return 'warning';
  return 'info';
}

const SEVERITY_STATUS: Record<Severity, Status> = { critical: 'down', warning: 'warning', info: 'info' };
const SEVERITY_LABEL: Record<Severity, string> = { critical: 'Critical', warning: 'Warning', info: 'Info' };

/** Severity presented through the status system (critical shares the down treatment). */
export function severityMeta(raw: string | null | undefined): StatusMeta {
  const severity = normalizeSeverity(raw);
  return { ...STATUS_META[SEVERITY_STATUS[severity]], label: SEVERITY_LABEL[severity] };
}

// ── Alert / incident lifecycle ─────────────────────────────────

const LIFECYCLE: Record<string, [Status, string]> = {
  active: ['down', 'Active'],
  open: ['down', 'Open'],
  firing: ['down', 'Firing'],
  triggered: ['down', 'Triggered'],
  acknowledged: ['warning', 'Acknowledged'],
  investigating: ['warning', 'Investigating'],
  identified: ['warning', 'Identified'],
  monitoring: ['info', 'Monitoring'],
  suppressed: ['maintenance', 'Suppressed'],
  resolved: ['up', 'Resolved'],
  closed: ['up', 'Closed'],
};

/** Presentation for alert/incident states (active, acknowledged, resolved...). */
export function lifecycleMeta(raw: string | null | undefined): StatusMeta {
  const key = (raw ?? '').toLowerCase();
  const entry = LIFECYCLE[key];
  if (!entry) return { ...STATUS_META.unknown, label: raw ? raw.charAt(0).toUpperCase() + raw.slice(1) : 'Unknown' };
  return { ...STATUS_META[entry[0]], label: entry[1] };
}

// ── Health score bands ─────────────────────────────────────────

export type HealthBand = 'healthy' | 'watch' | 'risk' | 'critical';

export interface HealthBandMeta {
  band: HealthBand;
  label: string;
  /** Presentation status: each band gets its own colour and shape. */
  status: Status;
  min: number;
}

/**
 * The one health-score banding, mirroring the backend's scoreToLabel()
 * (backend/internal/engine/health_scorer.go): <40 critical, <60 risk,
 * <80 watch, else healthy. Every gauge, insight and table uses this.
 */
export const HEALTH_BANDS: readonly HealthBandMeta[] = [
  { band: 'healthy', label: 'Healthy', status: 'up', min: 80 },
  { band: 'watch', label: 'Watch', status: 'info', min: 60 },
  { band: 'risk', label: 'Risk', status: 'warning', min: 40 },
  { band: 'critical', label: 'Critical', status: 'down', min: -Infinity },
];

export function healthBand(score: number): HealthBandMeta {
  const value = Number.isFinite(score) ? score : 0;
  return HEALTH_BANDS.find((b) => value >= b.min) ?? HEALTH_BANDS[HEALTH_BANDS.length - 1]!;
}

export function healthBandMeta(band: string): HealthBandMeta {
  return HEALTH_BANDS.find((b) => b.band === band) ?? HEALTH_BANDS[HEALTH_BANDS.length - 1]!;
}

/** A StatusMeta for a health score, labelled with its band ("Risk"). */
export function healthMeta(score: number): StatusMeta {
  const band = healthBand(score);
  return { ...STATUS_META[band.status], label: band.label };
}

/** A StatusMeta for a band name from the API ("critical" | "risk" | ...). */
export function healthBandStatusMeta(band: string): StatusMeta {
  const meta = healthBandMeta(band);
  return { ...STATUS_META[meta.status], label: meta.label };
}

/** Worst first: the order an operator wants to see problems in. */
export const STATUS_SEVERITY_ORDER: readonly Status[] = ['down', 'warning', 'unknown', 'maintenance', 'info', 'up'];

/** Position in STATUS_SEVERITY_ORDER (0 = down); for sorting worst-first. */
export function statusRank(raw: string | null | undefined): number {
  return STATUS_SEVERITY_ORDER.indexOf(normalizeStatus(raw));
}
