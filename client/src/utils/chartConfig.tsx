import type { CSSProperties, ReactNode } from 'react';

export const TOOLTIP_STYLE: CSSProperties = {
  background: 'var(--color-surface-container)',
  border: '1px solid var(--color-outline-variant)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-on-surface)',
};

export const DEVICE_COLORS = ['#d9fd3a', '#ff7351', '#6bb8c9', '#c084fc', '#8cc63f', '#fb923c'];

export const CHART_COLORS = ['#6bb8c9', '#d9fd3a', '#e5a910', '#c084fc', '#fb923c', '#8cc63f', '#f472b6', '#ff7351'];

export const PROTOCOL_COLORS: Record<string, string> = {
  TCP: '#6bb8c9',
  UDP: '#d9fd3a',
  ICMP: '#e5a910',
  IGMP: '#c084fc',
  GRE: '#fb923c',
  SCTP: '#8cc63f',
  ESP: '#f472b6',
};

export const AXIS_TICK_STYLE = { fill: '#77766d', fontSize: 10 };

export const LEGEND_STYLE = { fontSize: 11, paddingTop: 8 };

export function legendFormatter(value: string): ReactNode {
  return <span style={{ color: '#adaba1' }}>{value}</span>;
}
