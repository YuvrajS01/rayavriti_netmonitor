import type { ReactNode } from 'react';
import { STATUS_META, statusMeta, type StatusMeta, type StatusShape } from '../../utils/status';

/** Draws a status shape in currentColor. Decorative unless `label` is given. */
export function StatusShapeIcon({ shape, size = 10, label }: { shape: StatusShape; size?: number; label?: string }) {
  const a11y = label ? { role: 'img', 'aria-label': label } : { 'aria-hidden': true as const };
  return (
    <svg width={size} height={size} viewBox="0 0 10 10" className="shrink-0" {...a11y}>
      {shape === 'circle' && <circle cx="5" cy="5" r="4.5" fill="currentColor" />}
      {shape === 'triangle' && <path d="M5 0.6 9.6 9.2H0.4Z" fill="currentColor" />}
      {shape === 'square' && <rect x="1" y="1" width="8" height="8" rx="1" fill="currentColor" />}
      {shape === 'ring' && <circle cx="5" cy="5" r="3.75" fill="none" stroke="currentColor" strokeWidth="1.8" />}
      {shape === 'pause' && (
        <>
          <rect x="1.5" y="1" width="2.6" height="8" rx="0.6" fill="currentColor" />
          <rect x="5.9" y="1" width="2.6" height="8" rx="0.6" fill="currentColor" />
        </>
      )}
      {shape === 'dash' && <rect x="0.5" y="3.6" width="9" height="2.8" rx="1.4" fill="currentColor" />}
    </svg>
  );
}

function resolve(status: string | StatusMeta): StatusMeta {
  if (typeof status === 'object') return status;
  return (STATUS_META as Record<string, StatusMeta>)[status] ?? statusMeta(status);
}

interface StatusDotProps {
  /** A raw backend status, a normalized Status, or a meta from severityMeta()/healthBand. */
  status: string | StatusMeta;
  size?: number;
  /** Accessible name; defaults to the status label. Pass '' when a visible label sits next to it. */
  label?: string;
  className?: string;
}

/** Colour + shape status marker. Announces its label unless told it is decorative. */
export function StatusDot({ status, size = 10, label, className = '' }: StatusDotProps) {
  const meta = resolve(status);
  const name = label ?? meta.label;
  return (
    <span className={`inline-flex items-center ${meta.text} ${className}`} title={name || undefined}>
      <StatusShapeIcon shape={meta.shape} size={size} label={name || undefined} />
    </span>
  );
}

interface StatusPillProps {
  status: string | StatusMeta;
  /** Overrides the default label (e.g. "3 down", "SLA met"). */
  children?: ReactNode;
  size?: 'sm' | 'md';
  className?: string;
}

/** Tinted chip with shape + text. Text colour/background pairs are contrast-tested. */
export function StatusPill({ status, children, size = 'sm', className = '' }: StatusPillProps) {
  const meta = resolve(status);
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-1.5 rounded-full font-medium whitespace-nowrap ${meta.chip} ${meta.text} ${
        size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'
      } ${className}`}
    >
      <StatusShapeIcon shape={meta.shape} size={size === 'sm' ? 8 : 10} />
      {children ?? meta.label}
    </span>
  );
}
