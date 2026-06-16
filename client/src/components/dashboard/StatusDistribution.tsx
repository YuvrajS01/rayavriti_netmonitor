import { memo, useMemo } from 'react';
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

  const counts: Record<string, number> = { up: 0, warning: 0, down: 0, unknown: 0 };
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

function DonutCenter({ cx, cy, total }: { cx: number; cy: number; total: number }) {
  return (
    <text x={cx} y={cy} textAnchor="middle" dominantBaseline="middle" fill="#f4f1e6">
      <tspan x={cx} dy="-0.4em" fontSize="22" fontWeight="bold" fontFamily="'Space Grotesk', sans-serif">{total}</tspan>
      <tspan x={cx} dy="1.4em" fontSize="10" fill="#8a8a78" fontFamily="'Space Grotesk', sans-serif">DEVICES</tspan>
    </text>
  );
}

function StatusDistributionInner({ metrics }: Props) {
  const donutData = useMemo(() => buildDonutData(metrics), [metrics]);
  const donutTotal = useMemo(() => donutData.reduce((s, d) => s + d.value, 0), [donutData]);

  return (
    <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 flex flex-col">
      <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-widest">Status Distribution</h3>
      {donutTotal === 0 ? (
        <p className="text-xs text-on-surface-variant text-center my-auto py-8">No data yet</p>
      ) : (
        <div className="flex flex-col items-center justify-center flex-1">
          <ResponsiveContainer width="100%" height={180}>
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
              >
                {donutData.map((entry) => (
                  <Cell key={entry.name} fill={entry.color} stroke="transparent" />
                ))}
                <DonutCenter cx={0} cy={0} total={donutTotal} />
              </Pie>
              <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown, name: unknown) => [Number(v ?? 0), String(name)]} />
            </PieChart>
          </ResponsiveContainer>
          <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
            {donutData.map((d) => (
              <div key={d.name} className="flex items-center gap-1.5 text-xs">
                <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ background: d.color }} />
                <span className="text-on-surface-variant">{d.name}</span>
                <span className="font-bold text-on-surface">{d.value}</span>
              </div>
            ))}
          </div>
          <div className="sr-only">
            <ChartDataTable
              title="Status Distribution"
              columns={['Status', 'Count']}
              rows={donutData.map((d) => [d.name, d.value])}
            />
          </div>
        </div>
      )}
    </div>
  );
}

export const StatusDistribution = memo(StatusDistributionInner);
