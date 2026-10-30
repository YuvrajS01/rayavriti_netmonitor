import { describe, expect, it } from 'vitest';
import {
  HEALTH_BANDS,
  STATUS_META,
  healthBand,
  healthMeta,
  lifecycleMeta,
  normalizeSeverity,
  normalizeStatus,
  severityMeta,
  statusColorVar,
  statusIcon,
  statusLabel,
  statusMeta,
  statusTextClass,
} from '../status';
import { dedupeInsights } from '../insights';
import type { DeviceHealth } from '../../api/types';

describe('normalizeStatus', () => {
  it.each([
    ['up', 'up'], ['ok', 'up'], ['healthy', 'up'], ['ONLINE', 'up'],
    ['warning', 'warning'], ['degraded', 'warning'],
    ['down', 'down'], ['offline', 'down'], ['critical', 'down'],
    ['maintenance', 'maintenance'], ['paused', 'maintenance'],
    ['info', 'info'],
    ['unknown', 'unknown'], ['bogus', 'unknown'], ['', 'unknown'], [null, 'unknown'], [undefined, 'unknown'],
  ] as const)('%s → %s', (raw, expected) => {
    expect(normalizeStatus(raw)).toBe(expected);
  });
});

describe('STATUS_META', () => {
  it('gives every status a distinct shape and icon (colour is never the only signal)', () => {
    const metas = Object.values(STATUS_META);
    expect(new Set(metas.map((m) => m.shape)).size).toBe(metas.length);
    expect(new Set(metas.map((m) => m.icon)).size).toBe(metas.length);
  });

  it('uses status tokens only', () => {
    for (const meta of Object.values(STATUS_META)) {
      expect(meta.token).toBe(`status-${meta.status}`);
      expect(meta.text).toBe(`text-status-${meta.status}`);
    }
  });
});

describe('status helpers', () => {
  it('statusIcon("unknown") is not a success icon', () => {
    expect(statusIcon('unknown')).toBe('help');
    expect(statusIcon('unknown')).not.toBe(statusIcon('up'));
  });

  it('labels and classes', () => {
    expect(statusLabel('ok')).toBe('Up');
    expect(statusLabel('degraded')).toBe('Warning');
    expect(statusTextClass('down')).toBe('text-status-down');
    expect(statusColorVar('warning')).toBe('var(--color-status-warning)');
    expect(statusMeta('nope').status).toBe('unknown');
  });
});

describe('severity', () => {
  it('normalizes', () => {
    expect(normalizeSeverity('critical')).toBe('critical');
    expect(normalizeSeverity('warn')).toBe('warning');
    expect(normalizeSeverity('minor')).toBe('info');
  });
  it('critical shares the down treatment with its own label', () => {
    const meta = severityMeta('critical');
    expect(meta.status).toBe('down');
    expect(meta.label).toBe('Critical');
  });
});

describe('lifecycleMeta', () => {
  it('maps alert and incident states', () => {
    expect(lifecycleMeta('active').status).toBe('down');
    expect(lifecycleMeta('acknowledged').status).toBe('warning');
    expect(lifecycleMeta('resolved').status).toBe('up');
    expect(lifecycleMeta('weird')).toMatchObject({ status: 'unknown', label: 'Weird' });
  });
});

describe('healthBand', () => {
  it('matches the backend scoreToLabel thresholds', () => {
    expect(healthBand(95).band).toBe('healthy');
    expect(healthBand(80).band).toBe('healthy');
    expect(healthBand(79.9).band).toBe('watch');
    expect(healthBand(60).band).toBe('watch');
    expect(healthBand(59).band).toBe('risk');
    expect(healthBand(40).band).toBe('risk');
    expect(healthBand(39).band).toBe('critical');
    expect(healthBand(Number.NaN).band).toBe('critical');
  });

  it('gives each band its own status colour (critical and risk no longer collide)', () => {
    expect(new Set(HEALTH_BANDS.map((b) => b.status)).size).toBe(HEALTH_BANDS.length);
  });

  it('healthMeta labels the pill with the band', () => {
    expect(healthMeta(50)).toMatchObject({ status: 'warning', label: 'Risk' });
  });
});

describe('dedupeInsights', () => {
  const item = (deviceId: number, score: number) => ({ deviceId, deviceName: `d${deviceId}`, score }) as DeviceHealth;
  it('keeps one entry per device (the worst) sorted worst first', () => {
    const out = dedupeInsights([item(1, 70), item(2, 50), item(1, 40), item(3, 90)]);
    expect(out.map((i) => [i.deviceId, i.score])).toEqual([[1, 40], [2, 50], [3, 90]]);
  });
});
