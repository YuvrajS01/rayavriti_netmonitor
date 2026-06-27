import type { CSSProperties, ReactNode } from 'react';

export const TOOLTIP_STYLE: CSSProperties = {
  background: 'var(--color-surface-container)',
  border: '1px solid var(--color-outline-variant)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-on-surface)',
};

export const DEVICE_COLORS = ['var(--color-chart-1)', 'var(--color-error)', 'var(--color-chart-2)', 'var(--color-chart-3)', 'var(--color-chart-5)', '#fb923c'];

export const CHART_COLORS = ['var(--color-chart-2)', 'var(--color-chart-1)', 'var(--color-chart-4)', 'var(--color-chart-3)', '#fb923c', 'var(--color-chart-5)', 'var(--color-chart-7)', 'var(--color-error)'];

export const PROTOCOL_COLORS: Record<string, string> = {
  TCP: 'var(--color-chart-2)',
  UDP: 'var(--color-chart-1)',
  ICMP: 'var(--color-chart-4)',
  IGMP: 'var(--color-chart-3)',
  GRE: '#fb923c',
  SCTP: 'var(--color-chart-5)',
  ESP: 'var(--color-chart-7)',
};

export const AXIS_TICK_STYLE = { fill: 'var(--color-outline)', fontSize: 10 };

export const LEGEND_STYLE = { fontSize: 11, paddingTop: 8 };

export function legendFormatter(value: string): ReactNode {
  return <span style={{ color: 'var(--color-on-surface-variant)' }}>{value}</span>;
}
