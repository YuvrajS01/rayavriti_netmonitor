import type { CSSProperties, ReactNode } from 'react';

export const TOOLTIP_STYLE: CSSProperties = {
  background: 'var(--color-surface-container)',
  border: '1px solid var(--color-outline-variant)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-on-surface)',
};

export const DEVICE_COLORS = ['#d9fd3a', '#ff7351', '#6ee7f7', '#c084fc', '#4ade80', '#fb923c'];

export const CHART_COLORS = ['#6ee7f7', '#d9fd3a', '#f59e0b', '#c084fc', '#fb923c', '#4ade80', '#f472b6', '#ff7351'];

export const PROTOCOL_COLORS: Record<string, string> = {
  TCP: '#6ee7f7',
  UDP: '#d9fd3a',
  ICMP: '#f59e0b',
  IGMP: '#c084fc',
  GRE: '#fb923c',
  SCTP: '#4ade80',
  ESP: '#f472b6',
};

export const AXIS_TICK_STYLE = { fill: '#8a8a78', fontSize: 10 };

export const LEGEND_STYLE = { fontSize: 11, paddingTop: 8 };

export function legendFormatter(value: string): ReactNode {
  return <span style={{ color: '#c8c5b0' }}>{value}</span>;
}
