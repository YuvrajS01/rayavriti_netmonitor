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
  it('returns lime for up', () => {
    expect(statusHexColor('up')).toBe('#d9fd3a');
  });
  it('returns lime for ok', () => {
    expect(statusHexColor('ok')).toBe('#d9fd3a');
  });
  it('returns warning color for warning', () => {
    expect(statusHexColor('warning')).toBe('#e5a910');
  });
  it('returns warning color for degraded', () => {
    expect(statusHexColor('degraded')).toBe('#e5a910');
  });
  it('returns error color for down', () => {
    expect(statusHexColor('down')).toBe('#ff7351');
  });
  it('returns gray for unknown', () => {
    expect(statusHexColor('unknown')).toBe('#6b7280');
  });
  it('returns gray for unrecognized status', () => {
    expect(statusHexColor('bogus')).toBe('#6b7280');
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
  it('returns text-primary for up', () => {
    expect(statusTextColor('up')).toBe('text-primary');
  });
});

describe('statusBgColor', () => {
  it('returns bg-error for down', () => {
    expect(statusBgColor('down')).toBe('bg-error');
  });
  it('returns bg-warning for warning', () => {
    expect(statusBgColor('warning')).toBe('bg-warning');
  });
  it('returns bg-primary for up', () => {
    expect(statusBgColor('up')).toBe('bg-primary');
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
  it('returns border-primary for up', () => {
    expect(statusBorderColor('up')).toBe('border-primary');
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
  it('returns text-primary for minor', () => {
    expect(severityTextColor('minor')).toBe('text-primary');
  });
});

describe('severityBgColor', () => {
  it('returns bg-error/10 for critical', () => {
    expect(severityBgColor('critical')).toBe('bg-error/10');
  });
  it('returns bg-warning/10 for warning', () => {
    expect(severityBgColor('warning')).toBe('bg-warning/10');
  });
  it('returns bg-primary/10 for minor', () => {
    expect(severityBgColor('minor')).toBe('bg-primary/10');
  });
});

describe('severityBorderColor', () => {
  it('returns border-error for critical', () => {
    expect(severityBorderColor('critical')).toBe('border-error');
  });
  it('returns border-warning for warning', () => {
    expect(severityBorderColor('warning')).toBe('border-warning');
  });
  it('returns border-primary for minor', () => {
    expect(severityBorderColor('minor')).toBe('border-primary');
  });
});

describe('STATUS_COLORS', () => {
  it('exports status color map', () => {
    expect(STATUS_COLORS.up).toBe('#d9fd3a');
    expect(STATUS_COLORS.down).toBe('#ff7351');
  });
});

describe('STATUS_LABELS', () => {
  it('exports status label map', () => {
    expect(STATUS_LABELS.up).toBe('Healthy');
    expect(STATUS_LABELS.down).toBe('Down');
  });
});
