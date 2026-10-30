import AnimatedCounter from './AnimatedCounter';
import { StatusShapeIcon } from './Status';
import { STATUS_META, healthBand } from '../../utils/status';

interface Props {
  value: number;
  size?: number;
  strokeWidth?: number;
  /** Caption under the number (inside the ring). */
  label?: string;
  className?: string;
  decimals?: number;
  valueClassName?: string;
  /**
   * `health` (default): colour by the shared health bands and show the band
   * name. `neutral`: an accent-coloured ring for non-health percentages.
   */
  tone?: 'health' | 'neutral';
}

/** Circular 0–100 gauge. Health gauges use healthBand(), the same bands as everywhere else. */
export default function RingGauge({ value, size = 128, strokeWidth = 9, label, className, decimals, valueClassName, tone = 'health' }: Props) {
  const score = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - score / 100);
  const band = healthBand(score);
  const meta = STATUS_META[band.status];
  const color = tone === 'health' ? `var(--color-${meta.token})` : 'var(--color-primary)';
  const shownDecimals = decimals ?? (score % 1 ? 1 : 0);
  const large = size >= 96;
  return (
    <div className={`relative inline-grid place-items-center ${className ?? ''}`} style={{ width: size, height: size }}>
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="-rotate-90"
        role="img"
        aria-label={`${label ? `${label} ` : ''}${score.toFixed(shownDecimals)} of 100${tone === 'health' ? `, ${band.label}` : ''}`}
      >
        <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke="var(--color-surface-3)" strokeWidth={strokeWidth} />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          className="gauge-ring"
          style={{ '--gauge-circumference': circumference, '--gauge-offset': offset, strokeDashoffset: offset } as React.CSSProperties}
        />
      </svg>
      <div className="absolute flex flex-col items-center leading-none" aria-hidden="true">
        <span className={`font-semibold tabular-nums text-fg ${valueClassName ?? 'text-3xl'}`}>
          <AnimatedCounter value={score} decimals={shownDecimals} />
        </span>
        {large && (label || tone === 'health') && (
          <span className={`mt-1.5 inline-flex items-center gap-1 text-xs ${tone === 'health' ? meta.text : 'text-fg-muted'}`}>
            {tone === 'health' && <StatusShapeIcon shape={meta.shape} size={8} />}
            <span className={tone === 'health' ? '' : 'text-fg-muted'}>{tone === 'health' ? band.label : label}</span>
          </span>
        )}
      </div>
    </div>
  );
}
