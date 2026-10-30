import AnimatedCounter, { type CounterFormat } from './AnimatedCounter';
import Sparkline from './Sparkline';

interface StatCardProps {
  label: string;
  value: string | number;
  color?: string;
  icon?: string;
  sparklineData?: number[];
  trend?: 'up' | 'down' | 'flat';
  delta?: string | number;
  format?: CounterFormat;
}

export default function StatCard({ label, value, color = 'text-fg', icon, sparklineData, trend, delta, format }: StatCardProps) {
  const numericValue = typeof value === 'number' ? value : Number.parseFloat(value);
  const inferredFormat: CounterFormat = format ?? (typeof value === 'string' && value.includes('%') ? 'percent' : typeof value === 'string' && value.endsWith('ms') ? 'ms' : 'number');
  const safeValue = Number.isFinite(numericValue);
  const sparkColor = trend === 'down' ? 'var(--color-error)' : trend === 'flat' ? 'var(--color-fg-muted)' : 'var(--color-success)';
  return (
    <div className="bg-surface-1 p-5 rounded-lg border border-line stat-card group">
      <div className="flex items-center gap-2 mb-2">
        {icon && <span className="material-symbols-outlined text-sm opacity-60">{icon}</span>}
        <p className="text-fg-muted text-xs font-sans font-medium">{label}</p>
        {(trend || delta) && <span className={`ml-auto inline-flex items-center gap-0.5 text-xs font-mono ${trend === 'down' ? 'text-error' : trend === 'up' ? 'text-success' : 'text-fg-muted'}`}>
          {trend && <span className="material-symbols-outlined text-sm">{trend === 'up' ? 'trending_up' : trend === 'down' ? 'trending_down' : 'trending_flat'}</span>}{delta}
        </span>}
      </div>
      <div className="flex items-end justify-between gap-2">
        <p className={`font-sans text-2xl font-semibold tabular-nums ${color}`}>{safeValue ? <AnimatedCounter value={numericValue} format={inferredFormat} decimals={!Number.isInteger(numericValue) ? 1 : 0} /> : value}</p>
        {sparklineData && <Sparkline data={sparklineData} color={sparkColor} />}
      </div>
    </div>
  );
}
