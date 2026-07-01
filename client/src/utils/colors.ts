const STATUS_COLORS: Record<string, string> = {
  up: '#6ec96e',
  ok: '#6ec96e',
  warning: '#cca040',
  degraded: '#cca040',
  down: '#d85050',
  unknown: '#707070',
};

const STATUS_LABELS: Record<string, string> = {
  up: 'Healthy',
  ok: 'Healthy',
  warning: 'Warning',
  degraded: 'Warning',
  down: 'Down',
  unknown: 'Unknown',
};

export type StatusKey = 'up' | 'ok' | 'warning' | 'degraded' | 'down' | 'unknown';

export function statusHexColor(status: string): string {
  return STATUS_COLORS[status] ?? STATUS_COLORS.unknown;
}

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

export function statusTextColor(status: string): string {
  if (status === 'down') return 'text-error';
  if (status === 'warning' || status === 'degraded') return 'text-warning';
  if (status === 'unknown') return 'text-outline';
  return 'text-success';
}

export function statusBgColor(status: string): string {
  if (status === 'down') return 'bg-error';
  if (status === 'warning' || status === 'degraded') return 'bg-warning';
  if (status === 'unknown') return 'bg-outline';
  return 'bg-success';
}

export function statusBorderColor(status: string): string {
  if (status === 'down') return 'border-error';
  if (status === 'warning' || status === 'degraded') return 'border-warning';
  if (status === 'unknown') return 'border-outline-variant';
  return 'border-success';
}

export function statusIcon(status: string): string {
  if (status === 'down') return 'cancel';
  if (status === 'warning' || status === 'degraded') return 'warning';
  return 'check_circle';
}

export function severityIcon(severity: string): string {
  if (severity === 'critical') return 'dangerous';
  if (severity === 'warning') return 'warning';
  return 'info';
}

export function severityTextColor(severity: string): string {
  if (severity === 'critical') return 'text-error';
  if (severity === 'warning') return 'text-warning';
  return 'text-info';
}

export function severityBgColor(severity: string): string {
  if (severity === 'critical') return 'bg-error/10';
  if (severity === 'warning') return 'bg-warning/10';
  return 'bg-info/10';
}

export function severityBorderColor(severity: string): string {
  if (severity === 'critical') return 'border-error';
  if (severity === 'warning') return 'border-warning';
  return 'border-info';
}

export { STATUS_COLORS, STATUS_LABELS };
