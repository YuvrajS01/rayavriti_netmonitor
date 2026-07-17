import { memo, useMemo } from 'react';
import RadarChart from '../charts/RadarChart';
import type { Metric } from '../../api/types';

interface Props {
  metrics: Metric[];
}

function AvgResponseByStatusInner({ metrics }: Props) {
  const { series, axes } = useMemo(() => {
    const groups = (['up', 'warning', 'down'] as const).map((s) => {
      const groupMetrics = metrics.filter((m) => {
        if (s === 'up') return m.status === 'up' || m.status === 'ok';
        if (s === 'warning') return m.status === 'warning' || m.status === 'degraded';
        return m.status === 'down';
      });
      const avg = groupMetrics.length
        ? groupMetrics.reduce((acc, m) => acc + (m.responseTime || 0), 0) / groupMetrics.length
        : 0;
      const max = Math.max(1, ...metrics.map((m) => m.responseTime || 0));
      const labels = s === 'up' ? 'Healthy' : s === 'warning' ? 'Warning' : 'Down';
      return { label: labels, count: groupMetrics.length, latencyScore: Math.round((1 - Math.min(avg / max, 1)) * 100) };
    });
    const axes = ['Latency', 'Availability', 'Stability', 'Coverage', 'Health'];
    const series = groups.map((g, i) => {
      const availability = g.count ? Math.round((g.latencyScore + 30) * 0.7 + 30) : 0;
      const values = [
        g.latencyScore,
        availability,
        Math.round(g.latencyScore * 0.9),
        Math.min(100, g.count * 12),
        Math.round((g.latencyScore + availability) / 2),
      ];
      const color = i === 0 ? 'var(--color-success)' : i === 1 ? 'var(--color-warning)' : 'var(--color-error)';
      return { label: g.label, values, color };
    });
    return { series, axes };
  }, [metrics]);

  return (
    <div className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20">
      <h3 className="text-sm font-headline font-semibold text-on-surface mb-1">Avg Response by Status</h3>
      <p className="text-[10px] uppercase tracking-wide text-on-surface-variant mb-2">Multi-factor comparison</p>
      <div className="flex flex-col items-center">
        <RadarChart axes={axes} series={series} size={200} />
        <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
          {series.map((s) => (
            <div key={s.label} className="flex items-center gap-1.5 text-xs">
              <span className="w-2.5 h-2.5 rounded-full" style={{ background: s.color }} />
              <span className="text-on-surface-variant">{s.label}</span>
              <span className="font-semibold text-on-surface">{s.values[0]}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export const AvgResponseByStatus = memo(AvgResponseByStatusInner);
