import { STATUS_META, type StatusMeta } from '../../utils/status';

/**
 * Availability bands used across the report tabs (and the PDF/print view):
 * ≥99% healthy, ≥95% degraded, otherwise poor. Always rendered as a
 * StatusPill so the band is readable without colour.
 */
export function availabilityStatus(percent: number): StatusMeta {
  if (percent >= 99) return { ...STATUS_META.up, label: 'Healthy' };
  if (percent >= 95) return { ...STATUS_META.warning, label: 'Degraded' };
  return { ...STATUS_META.down, label: 'Poor' };
}
