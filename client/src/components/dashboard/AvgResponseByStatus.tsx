import { memo, useMemo } from 'react';
import type { Metric } from '../../api/types';

interface Props {
  metrics: Metric[];
}

const GROUPS = [
  { key: 'up', label: 'Healthy', color: 'var(--color-success)', match: (s: string) => s === 'up' || s === 'ok' },
  { key: 'warning', label: 'Warning', color: 'var(--color-warning)', match: (s: string) => s === 'warning' || s === 'degraded' },
  { key: 'down', label: 'Down', color: 'var(--color-error)', match: (s: string) => s === 'down' },
] as const;

// Average response time of each device's latest poll, grouped by status.
function AvgResponseByStatusInner({ metrics }: Props) {
  const rows = useMemo(() => {
    const groups = GROUPS.map((g) => {
      const timed = metrics.filter((m) => g.match(m.status) && m.responseTime != null);
      const count = metrics.filter((m) => g.match(m.status)).length;
      const avg = timed.length ? timed.reduce((acc, m) => acc + (m.responseTime ?? 0), 0) / timed.length : null;
      return { ...g, count, avg };
    });
    const max = Math.max(1, ...groups.map((g) => g.avg ?? 0));
    return groups.map((g) => ({ ...g, pct: g.avg == null ? 0 : (g.avg / max) * 100 }));
  }, [metrics]);

  return (
    <div className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20">
      <h3 className="text-sm font-headline font-semibold text-on-surface mb-1">Avg Response by Status</h3>
      <p className="text-xs text-on-surface-variant mb-4">Latest poll of each device</p>
      <div className="space-y-4">
        {rows.map((r) => (
          <div key={r.key}>
            <div className="flex justify-between text-xs mb-1">
              <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full" style={{ background: r.color }} />{r.label} <span className="text-on-surface-variant">({r.count})</span></span>
              <span className="font-data tabular-nums">{r.avg == null ? '—' : `${r.avg.toFixed(1)} ms`}</span>
            </div>
            <div className="h-2 bg-surface-container rounded overflow-hidden">
              <div className="h-2 rounded transition-[width] duration-500" style={{ width: `${r.pct}%`, background: r.color }} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export const AvgResponseByStatus = memo(AvgResponseByStatusInner);
