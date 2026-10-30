import { memo, useMemo, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';
import type { Metric } from '../../api/types';
import { STATUS_COLORS, STATUS_LABELS } from '../../utils/colors';
import { TOOLTIP_STYLE } from '../../utils/chartConfig';
import ChartDataTable from '../ui/ChartDataTable';

interface Props {
  metrics: Metric[];
}

interface DonutSlice { name: string; value: number; color: string }

function buildDonutData(metrics: Metric[]): DonutSlice[] {
  const byDevice = new Map<number, string>();
  for (const m of metrics) byDevice.set(m.deviceId, m.status);

  const counts = { up: 0, warning: 0, down: 0, unknown: 0 };
  for (const [, status] of byDevice) {
    if (status === 'up' || status === 'ok') counts.up++;
    else if (status === 'warning' || status === 'degraded') counts.warning++;
    else if (status === 'down') counts.down++;
    else counts.unknown++;
  }

  return Object.entries(counts)
    .filter(([, v]) => v > 0)
    .map(([name, value]) => ({
      name: STATUS_LABELS[name] ?? name,
      value,
      color: STATUS_COLORS[name] ?? '#6b7280',
    }));
}

function StatusDistributionInner({ metrics }: Props) {
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);
  const donutData = useMemo(() => buildDonutData(metrics), [metrics]);
  const donutTotal = useMemo(() => donutData.reduce((s, d) => s + d.value, 0), [donutData]);
  const active = activeIndex != null ? donutData[activeIndex] : undefined;

  return (
    <div className="bg-surface-1 rounded-lg p-4 border border-line flex flex-col group">
      <h3 className="text-sm font-sans font-semibold text-fg mb-3">Status Distribution</h3>
      {donutTotal === 0 ? (
        <p className="text-xs text-fg-muted text-center my-auto py-8">No data yet</p>
      ) : (
        <div className="flex flex-col items-center justify-center flex-1">
          <ResponsiveContainer width="100%" height={190}>
            <PieChart>
              <Pie
                data={donutData}
                cx="50%"
                cy="50%"
                innerRadius={54}
                outerRadius={78}
                paddingAngle={3}
                dataKey="value"
                labelLine={false}
                animationBegin={120}
                animationDuration={700}
              >
                {donutData.map((entry, i) => (
                  <Cell
                    key={entry.name}
                    fill={entry.color}
                    stroke="transparent"
                    onMouseEnter={() => setActiveIndex(i)}
                    onMouseLeave={() => setActiveIndex(undefined)}
                    style={{ filter: activeIndex === i ? `drop-shadow(0 0 6px ${entry.color})` : undefined, transition: 'filter 200ms', cursor: 'pointer', opacity: activeIndex != null && activeIndex !== i ? 0.55 : 1 }}
                  />
                ))}
              </Pie>
              <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown, name: unknown) => [Number(v ?? 0), String(name)]} />
              <text x="50%" y="46%" textAnchor="middle" dominantBaseline="middle" fill="currentColor" className="text-fg">
                <tspan x="50%" dy="-0.3em" fontSize="24" fontWeight="600" fontFamily="var(--font-sans)">{active ? active.value : donutTotal}</tspan>
                <tspan x="50%" dy="1.3em" fontSize="12" fill="var(--color-fg-muted)" fontFamily="var(--font-sans)">{active ? active.name : 'devices'}</tspan>
              </text>
            </PieChart>
          </ResponsiveContainer>
          <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
            {donutData.map((d, i) => (
              <div
                key={d.name}
                className={`flex items-center gap-1.5 text-xs transition-opacity ${activeIndex != null && activeIndex !== i ? 'opacity-50' : ''}`}
                onMouseEnter={() => setActiveIndex(i)}
                onMouseLeave={() => setActiveIndex(undefined)}
              >
                <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ background: d.color, boxShadow: activeIndex === i ? `0 0 8px ${d.color}` : undefined }} />
                <span className="text-fg-muted">{d.name}</span>
                <span className="font-semibold text-fg">{d.value}</span>
              </div>
            ))}
          </div>
          <div className="sr-only">
            <ChartDataTable title="Status Distribution" columns={['Status', 'Count']} rows={donutData.map((d) => [d.name, d.value])} />
          </div>
        </div>
      )}
    </div>
  );
}

export const StatusDistribution = memo(StatusDistributionInner);
