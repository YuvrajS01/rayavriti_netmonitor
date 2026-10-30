import { describe, expect, it } from 'vitest';
import { formatBps } from '../formatters';

describe('formatBps', () => {
  it('picks an SI unit and keeps three significant figures', () => {
    expect(formatBps(0)).toBe('0 bps');
    expect(formatBps(950)).toBe('950 bps');
    expect(formatBps(20_057)).toBe('20.1 Kbps');
    expect(formatBps(1_500_000)).toBe('1.50 Mbps');
    expect(formatBps(10_000_000_000)).toBe('10.0 Gbps');
  });
});
