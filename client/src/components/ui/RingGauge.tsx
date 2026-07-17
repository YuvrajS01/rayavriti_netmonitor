import AnimatedCounter from './AnimatedCounter';

interface Props { value: number; size?: number; strokeWidth?: number; label?: string; className?: string; showParticles?: boolean; decimals?: number; valueClassName?: string; }
export default function RingGauge({ value, size = 128, strokeWidth = 9, label, className, showParticles = false, decimals, valueClassName }: Props) {
  const score = Math.max(0, Math.min(100, value));
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference * (1 - score / 100);
  const color = score < 55 ? 'var(--color-error)' : score < 75 ? 'var(--color-warning)' : 'var(--color-success)';
  const shownDecimals = decimals ?? (score % 1 ? 1 : 0);
  return <div className={`relative inline-grid place-items-center ${className ?? ''}`} style={{ width: size, height: size }}>
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="-rotate-90" role="img" aria-label={`${label || 'Score'} ${score}%`}>
      <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke="var(--color-surface-container)" strokeWidth={strokeWidth} />
      <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke={color} strokeWidth={strokeWidth} strokeLinecap="round" strokeDasharray={circumference} className="gauge-ring" style={{ '--gauge-circumference': circumference, '--gauge-offset': offset, strokeDashoffset: offset } as React.CSSProperties} />
    </svg>
    {showParticles && score > 90 && <span className="absolute inset-4 rounded-full ring-particles" aria-hidden="true" />}
    <span className={`absolute font-headline font-semibold tabular-nums ${valueClassName ?? 'text-3xl'}`} style={{ color }}><AnimatedCounter value={score} decimals={shownDecimals} /></span>
    {label && <span className="absolute top-[68%] text-[9px] uppercase tracking-[.14em] text-on-surface-variant">{label}</span>}
  </div>;
}
