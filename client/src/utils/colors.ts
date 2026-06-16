const STATUS_COLORS: Record<string, string> = {
  up: '#d9fd3a',
  ok: '#d9fd3a',
  warning: '#f59e0b',
  degraded: '#f59e0b',
  down: '#ff4444',
  unknown: '#6b7280',
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
  if (status === 'warning' || status === 'degraded') return 'text-amber-400';
  if (status === 'unknown') return 'text-outline';
  return 'text-primary';
}

export function statusBgColor(status: string): string {
  if (status === 'down') return 'bg-error';
  if (status === 'warning' || status === 'degraded') return 'bg-amber-500';
  if (status === 'unknown') return 'bg-outline';
  return 'bg-primary';
}

export function statusBorderColor(status: string): string {
  if (status === 'down') return 'border-error';
  if (status === 'warning' || status === 'degraded') return 'border-amber-500';
  if (status === 'unknown') return 'border-outline-variant';
  return 'border-primary';
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
  if (severity === 'warning') return 'text-amber-500';
  return 'text-primary';
}

export function severityBgColor(severity: string): string {
  if (severity === 'critical') return 'bg-error/10';
  if (severity === 'warning') return 'bg-amber-500/10';
  return 'bg-primary/10';
}

export function severityBorderColor(severity: string): string {
  if (severity === 'critical') return 'border-error';
  if (severity === 'warning') return 'border-amber-500';
  return 'border-primary';
}

export { STATUS_COLORS, STATUS_LABELS };
