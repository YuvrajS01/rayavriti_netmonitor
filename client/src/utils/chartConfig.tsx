import type { CSSProperties, ReactNode } from 'react';

/**
 * Chart theme. Everything here references CSS custom properties, so recharts
 * (which writes them into SVG presentation attributes/styles) follows the
 * active theme without a re-render. Canvas code that cannot use var() reads
 * resolved values with `readToken()` from utils/tokens.ts.
 */

export const TOOLTIP_STYLE: CSSProperties = {
  background: 'var(--color-surface-2)',
  border: '1px solid var(--color-line-strong)',
  borderRadius: '8px',
  fontSize: '12px',
  color: 'var(--color-fg)',
  boxShadow: 'var(--shadow-e2)',
};

export const TOOLTIP_LABEL_STYLE: CSSProperties = { color: 'var(--color-fg-muted)' };
export const TOOLTIP_ITEM_STYLE: CSSProperties = { color: 'var(--color-fg)' };

/** The categorical palette, in its fixed order. Never cycled past 8. */
export const SERIES_COLORS = [
  'var(--color-chart-1)',
  'var(--color-chart-2)',
  'var(--color-chart-3)',
  'var(--color-chart-4)',
  'var(--color-chart-5)',
  'var(--color-chart-6)',
  'var(--color-chart-7)',
  'var(--color-chart-8)',
];

/** Per-device series (multi-line charts). */
export const DEVICE_COLORS = [...SERIES_COLORS];

/** Generic categorical series (pies, stacked bars). */
export const CHART_COLORS = [...SERIES_COLORS];

export const PROTOCOL_COLORS: Record<string, string> = {
  TCP: 'var(--color-chart-1)',
  UDP: 'var(--color-chart-2)',
  ICMP: 'var(--color-chart-3)',
  IGMP: 'var(--color-chart-4)',
  GRE: 'var(--color-chart-5)',
  SCTP: 'var(--color-chart-6)',
  ESP: 'var(--color-chart-7)',
};

/** Axis tick labels use the subtle text token (≥4.5:1), never a border colour. */
export const AXIS_TICK_STYLE = { fill: 'var(--color-fg-subtle)', fontSize: 12 };
export const AXIS_LINE_COLOR = 'var(--color-line-strong)';
export const GRID_COLOR = 'var(--color-line)';

export const LEGEND_STYLE = { fontSize: 12, paddingTop: 8 };

export function legendFormatter(value: string): ReactNode {
  return <span style={{ color: 'var(--color-fg-muted)' }}>{value}</span>;
}
