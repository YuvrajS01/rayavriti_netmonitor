import AnimatedCounter, { type CounterFormat } from './AnimatedCounter';
import Sparkline from './Sparkline';

interface StatCardProps {
  label: string;
  value: string | number;
  /** Text colour class for the value; use a status text class only when the number IS a status count. */
  color?: string;
  icon?: string;
  sparklineData?: number[];
  trend?: 'up' | 'down' | 'flat';
  delta?: string | number;
  format?: CounterFormat;
}

/**
 * A static KPI tile. Not interactive, so no hover lift: wrap it in a link
 * or button if it should navigate.
 */
export default function StatCard({ label, value, color = 'text-fg', icon, sparklineData, trend, delta, format }: StatCardProps) {
  const numericValue = typeof value === 'number' ? value : Number.parseFloat(value);
  const inferredFormat: CounterFormat = format ?? (typeof value === 'string' && value.includes('%') ? 'percent' : typeof value === 'string' && value.endsWith('ms') ? 'ms' : 'number');
  const safeValue = Number.isFinite(numericValue);
  return (
    <div className="bg-surface-1 p-card rounded-lg border border-line">
      <div className="flex items-center gap-2 mb-2">
        {icon && <span className="material-symbols-outlined text-[18px] text-fg-subtle" aria-hidden="true">{icon}</span>}
        <p className="text-fg-muted text-sm font-medium">{label}</p>
        {(trend || delta) && (
          <span className="ml-auto inline-flex items-center gap-0.5 text-xs text-fg-muted">
            {trend && (
              <span className="material-symbols-outlined text-sm" aria-label={`Trend ${trend}`} role="img">
                {trend === 'up' ? 'trending_up' : trend === 'down' ? 'trending_down' : 'trending_flat'}
              </span>
            )}
            {delta}
          </span>
        )}
      </div>
      <div className="flex items-end justify-between gap-2">
        <p className={`text-2xl font-semibold tabular-nums ${color}`}>
          {safeValue ? <AnimatedCounter value={numericValue} format={inferredFormat} decimals={!Number.isInteger(numericValue) ? 1 : 0} /> : value}
        </p>
        {sparklineData && <Sparkline data={sparklineData} color="var(--color-chart-1)" />}
      </div>
    </div>
  );
}
