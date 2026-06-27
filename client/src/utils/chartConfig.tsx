import type { CSSProperties, ReactNode } from 'react';

export const TOOLTIP_STYLE: CSSProperties = {
  background: 'var(--color-surface-container)',
  border: '1px solid var(--color-outline-variant)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-on-surface)',
};

export const DEVICE_COLORS = ['var(--color-primary)', 'var(--color-error)', 'var(--color-info)', '#c084fc', 'var(--color-success)', '#fb923c'];

export const CHART_COLORS = ['var(--color-info)', 'var(--color-primary)', 'var(--color-warning)', '#c084fc', '#fb923c', 'var(--color-success)', '#f472b6', 'var(--color-error)'];

export const PROTOCOL_COLORS: Record<string, string> = {
  TCP: 'var(--color-info)',
  UDP: 'var(--color-primary)',
  ICMP: 'var(--color-warning)',
  IGMP: '#c084fc',
  GRE: '#fb923c',
  SCTP: 'var(--color-success)',
  ESP: '#f472b6',
};

export const AXIS_TICK_STYLE = { fill: 'var(--color-outline)', fontSize: 10 };

export const LEGEND_STYLE = { fontSize: 11, paddingTop: 8 };

export function legendFormatter(value: string): ReactNode {
  return <span style={{ color: 'var(--color-on-surface-variant)' }}>{value}</span>;
}
