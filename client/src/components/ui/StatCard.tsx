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

export default function StatCard({ label, value, color = 'text-on-surface', icon, sparklineData, trend, delta, format }: StatCardProps) {
  const numericValue = typeof value === 'number' ? value : Number.parseFloat(value);
  const inferredFormat: CounterFormat = format ?? (typeof value === 'string' && value.includes('%') ? 'percent' : typeof value === 'string' && value.endsWith('ms') ? 'ms' : 'number');
  const safeValue = Number.isFinite(numericValue);
  const sparkColor = trend === 'down' ? 'var(--color-error)' : trend === 'flat' ? 'var(--color-on-surface-variant)' : 'var(--color-success)';
  return (
    <div className="bg-surface-container-low p-5 rounded-lg border border-outline-variant/20 stat-card group">
      <div className="flex items-center gap-2 mb-2">
        {icon && <span className="material-symbols-outlined text-sm opacity-60">{icon}</span>}
        <p className="text-on-surface-variant text-xs uppercase tracking-wide font-label font-medium">{label}</p>
        {(trend || delta) && <span className={`ml-auto inline-flex items-center gap-0.5 text-[10px] font-data ${trend === 'down' ? 'text-error' : trend === 'flat' ? 'text-on-surface-variant' : 'text-success'}`}>
          {trend && <span className="material-symbols-outlined text-sm">{trend === 'up' ? 'trending_up' : trend === 'down' ? 'trending_down' : 'trending_flat'}</span>}{delta}
        </span>}
      </div>
      <div className="flex items-end justify-between gap-2">
        <p className={`font-headline text-2xl font-semibold tabular-nums ${color}`}>{safeValue ? <AnimatedCounter value={numericValue} format={inferredFormat} decimals={!Number.isInteger(numericValue) ? 1 : 0} /> : value}</p>
        {sparklineData && <Sparkline data={sparklineData} color={sparkColor} />}
      </div>
    </div>
  );
}
