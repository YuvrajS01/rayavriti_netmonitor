import { memo, useMemo } from 'react';
import type { Metric } from '../../api/types';
import { STATUS_META, normalizeStatus, statusColorVar, type Status } from '../../utils/status';
import { StatusShapeIcon } from '../ui/Status';

interface Props {
  metrics: Metric[];
}

const GROUPS: Status[] = ['up', 'warning', 'down'];

// Average response time of each device's latest poll, grouped by status.
function AvgResponseByStatusInner({ metrics }: Props) {
  const rows = useMemo(() => {
    const groups = GROUPS.map((status) => {
      const matching = metrics.filter((m) => normalizeStatus(m.status) === status);
      const timed = matching.filter((m) => m.responseTime != null);
      const avg = timed.length ? timed.reduce((acc, m) => acc + (m.responseTime ?? 0), 0) / timed.length : null;
      return { status, meta: STATUS_META[status], count: matching.length, avg };
    });
    const max = Math.max(1, ...groups.map((g) => g.avg ?? 0));
    return groups.map((g) => ({ ...g, pct: g.avg == null ? 0 : (g.avg / max) * 100 }));
  }, [metrics]);

  return (
    <section aria-labelledby="avg-response-heading" className="bg-surface-1 rounded-lg p-5 border border-line">
      <h3 id="avg-response-heading" className="text-sm font-semibold text-fg mb-1">Avg response by status</h3>
      <p className="text-xs text-fg-muted mb-4">Latest poll of each device</p>
      <div className="space-y-4">
        {rows.map((r) => (
          <div key={r.status}>
            <div className="flex justify-between text-xs mb-1">
              <span className="flex items-center gap-1.5 text-fg">
                <span className={r.meta.text}><StatusShapeIcon shape={r.meta.shape} /></span>
                {r.meta.label} <span className="text-fg-muted">({r.count})</span>
              </span>
              <span className="font-mono text-fg">{r.avg == null ? '—' : `${r.avg.toFixed(1)} ms`}</span>
            </div>
            <div className="h-2 bg-surface-3 rounded-sm overflow-hidden">
              <div className="h-2 rounded-sm transition-[width] duration-[var(--duration-normal)]" style={{ width: `${r.pct}%`, background: statusColorVar(r.status) }} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

export const AvgResponseByStatus = memo(AvgResponseByStatusInner);
