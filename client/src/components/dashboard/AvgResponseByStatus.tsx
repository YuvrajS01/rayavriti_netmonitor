import { memo, useMemo } from 'react';
import { STATUS_COLORS } from '../../utils/colors';
import type { Metric } from '../../api/types';

interface Props {
  metrics: Metric[];
}

function AvgResponseByStatusInner({ metrics }: Props) {
  const rows = useMemo(() => {
    return (['up', 'warning', 'down'] as const).map((s) => {
      const statusMetrics = metrics.filter((m) => {
        if (s === 'up') return m.status === 'up' || m.status === 'ok';
        if (s === 'warning') return m.status === 'warning' || m.status === 'degraded';
        return m.status === 'down';
      });
      const avg = statusMetrics.length
        ? Math.round(statusMetrics.reduce((acc, m) => acc + (m.responseTime || 0), 0) / statusMetrics.length)
        : 0;
      const label = s === 'up' ? 'Healthy' : s === 'warning' ? 'Warning' : 'Down';
      const color = STATUS_COLORS[s];
      const barMax = 2000;
      const barWidth = Math.min(100, (avg / barMax) * 100);
      return { key: s, label, color, avg, count: statusMetrics.length, barWidth };
    });
  }, [metrics]);

  return (
    <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20">
      <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-wide">Avg Response by Status</h3>
      <div className="space-y-3 mt-2">
        {rows.map((r) => (
          <div key={r.key}>
            <div className="flex justify-between text-xs mb-1">
              <span style={{ color: r.color }}>{r.label} ({r.count} device{r.count !== 1 ? 's' : ''})</span>
              <span className="text-on-surface-variant">{r.avg}ms</span>
            </div>
            <div className="h-2 bg-surface-container-highest rounded">
              <div className="h-2 rounded transition-[width] duration-500" style={{ width: `${r.barWidth}%`, background: r.color }} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export const AvgResponseByStatus = memo(AvgResponseByStatusInner);
