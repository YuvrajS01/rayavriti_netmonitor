import { describe, it, expect } from 'vitest';
import {
  TOOLTIP_STYLE,
  DEVICE_COLORS,
  CHART_COLORS,
  PROTOCOL_COLORS,
  AXIS_TICK_STYLE,
  LEGEND_STYLE,
} from '../chartConfig';

describe('TOOLTIP_STYLE', () => {
  it('has background, border, borderRadius, fontSize, color', () => {
    expect(TOOLTIP_STYLE.background).toBeDefined();
    expect(TOOLTIP_STYLE.border).toBeDefined();
    expect(TOOLTIP_STYLE.borderRadius).toBe('8px');
    expect(TOOLTIP_STYLE.fontSize).toBe('12px');
    expect(TOOLTIP_STYLE.color).toBeDefined();
  });
});

describe('DEVICE_COLORS', () => {
  it('has at least 6 colors', () => {
    expect(DEVICE_COLORS.length).toBeGreaterThanOrEqual(6);
  });
  it('starts with primary', () => {
    expect(DEVICE_COLORS[0]).toBe('var(--color-chart-1)');
  });
});

describe('CHART_COLORS', () => {
  it('has at least 6 colors', () => {
    expect(CHART_COLORS.length).toBeGreaterThanOrEqual(6);
  });
});

describe('PROTOCOL_COLORS', () => {
  it('maps TCP, UDP, ICMP', () => {
    expect(PROTOCOL_COLORS.TCP).toBe('var(--color-chart-2)');
    expect(PROTOCOL_COLORS.UDP).toBe('var(--color-chart-1)');
    expect(PROTOCOL_COLORS.ICMP).toBe('var(--color-chart-4)');
  });
});

describe('AXIS_TICK_STYLE', () => {
  it('has fill and fontSize', () => {
    expect(AXIS_TICK_STYLE.fill).toBeDefined();
    expect(AXIS_TICK_STYLE.fontSize).toBe(10);
  });
});

describe('LEGEND_STYLE', () => {
  it('has fontSize and paddingTop', () => {
    expect(LEGEND_STYLE.fontSize).toBe(11);
    expect(LEGEND_STYLE.paddingTop).toBe(8);
  });
});
