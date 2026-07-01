import { describe, it, expect } from 'vitest';
import {
  statusHexColor,
  statusLabel,
  statusTextColor,
  statusBgColor,
  statusBorderColor,
  statusIcon,
  severityIcon,
  severityTextColor,
  severityBgColor,
  severityBorderColor,
  STATUS_COLORS,
  STATUS_LABELS,
} from '../colors';

describe('statusHexColor', () => {
  it('returns green for up', () => {
    expect(statusHexColor('up')).toBe('#6ec96e');
  });
  it('returns green for ok', () => {
    expect(statusHexColor('ok')).toBe('#6ec96e');
  });
  it('returns warning color for warning', () => {
    expect(statusHexColor('warning')).toBe('#cca040');
  });
  it('returns warning color for degraded', () => {
    expect(statusHexColor('degraded')).toBe('#cca040');
  });
  it('returns error color for down', () => {
    expect(statusHexColor('down')).toBe('#d85050');
  });
  it('returns gray for unknown', () => {
    expect(statusHexColor('unknown')).toBe('#707070');
  });
  it('returns gray for unrecognized status', () => {
    expect(statusHexColor('bogus')).toBe('#707070');
  });
});

describe('statusLabel', () => {
  it('returns Healthy for up', () => {
    expect(statusLabel('up')).toBe('Healthy');
  });
  it('returns Healthy for ok', () => {
    expect(statusLabel('ok')).toBe('Healthy');
  });
  it('returns Warning for warning', () => {
    expect(statusLabel('warning')).toBe('Warning');
  });
  it('returns Warning for degraded', () => {
    expect(statusLabel('degraded')).toBe('Warning');
  });
  it('returns Down for down', () => {
    expect(statusLabel('down')).toBe('Down');
  });
  it('returns raw value for unknown', () => {
    expect(statusLabel('foo')).toBe('foo');
  });
});

describe('statusTextColor', () => {
  it('returns text-error for down', () => {
    expect(statusTextColor('down')).toBe('text-error');
  });
  it('returns text-warning for warning', () => {
    expect(statusTextColor('warning')).toBe('text-warning');
  });
  it('returns text-warning for degraded', () => {
    expect(statusTextColor('degraded')).toBe('text-warning');
  });
  it('returns text-outline for unknown', () => {
    expect(statusTextColor('unknown')).toBe('text-outline');
  });
  it('returns text-success for up', () => {
    expect(statusTextColor('up')).toBe('text-success');
  });
});

describe('statusBgColor', () => {
  it('returns bg-error for down', () => {
    expect(statusBgColor('down')).toBe('bg-error');
  });
  it('returns bg-warning for warning', () => {
    expect(statusBgColor('warning')).toBe('bg-warning');
  });
  it('returns bg-success for up', () => {
    expect(statusBgColor('up')).toBe('bg-success');
  });
});

describe('statusBorderColor', () => {
  it('returns border-error for down', () => {
    expect(statusBorderColor('down')).toBe('border-error');
  });
  it('returns border-warning for degraded', () => {
    expect(statusBorderColor('degraded')).toBe('border-warning');
  });
  it('returns border-outline-variant for unknown', () => {
    expect(statusBorderColor('unknown')).toBe('border-outline-variant');
  });
  it('returns border-success for up', () => {
    expect(statusBorderColor('up')).toBe('border-success');
  });
});

describe('statusIcon', () => {
  it('returns cancel for down', () => {
    expect(statusIcon('down')).toBe('cancel');
  });
  it('returns warning for warning', () => {
    expect(statusIcon('warning')).toBe('warning');
  });
  it('returns warning for degraded', () => {
    expect(statusIcon('degraded')).toBe('warning');
  });
  it('returns check_circle for up', () => {
    expect(statusIcon('up')).toBe('check_circle');
  });
});

describe('severityIcon', () => {
  it('returns dangerous for critical', () => {
    expect(severityIcon('critical')).toBe('dangerous');
  });
  it('returns warning for warning', () => {
    expect(severityIcon('warning')).toBe('warning');
  });
  it('returns info for minor', () => {
    expect(severityIcon('minor')).toBe('info');
  });
});

describe('severityTextColor', () => {
  it('returns text-error for critical', () => {
    expect(severityTextColor('critical')).toBe('text-error');
  });
  it('returns text-warning for warning', () => {
    expect(severityTextColor('warning')).toBe('text-warning');
  });
  it('returns text-info for minor', () => {
    expect(severityTextColor('minor')).toBe('text-info');
  });
});

describe('severityBgColor', () => {
  it('returns bg-error/10 for critical', () => {
    expect(severityBgColor('critical')).toBe('bg-error/10');
  });
  it('returns bg-warning/10 for warning', () => {
    expect(severityBgColor('warning')).toBe('bg-warning/10');
  });
  it('returns bg-info/10 for minor', () => {
    expect(severityBgColor('minor')).toBe('bg-info/10');
  });
});

describe('severityBorderColor', () => {
  it('returns border-error for critical', () => {
    expect(severityBorderColor('critical')).toBe('border-error');
  });
  it('returns border-warning for warning', () => {
    expect(severityBorderColor('warning')).toBe('border-warning');
  });
  it('returns border-info for minor', () => {
    expect(severityBorderColor('minor')).toBe('border-info');
  });
});

describe('STATUS_COLORS', () => {
  it('exports status color map', () => {
    expect(STATUS_COLORS.up).toBe('#6ec96e');
    expect(STATUS_COLORS.down).toBe('#d85050');
  });
});

describe('STATUS_LABELS', () => {
  it('exports status label map', () => {
    expect(STATUS_LABELS.up).toBe('Healthy');
    expect(STATUS_LABELS.down).toBe('Down');
  });
});
