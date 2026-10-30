import { memo, useMemo, useState } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';
import type { Metric } from '../../api/types';
import { STATUS_META, normalizeStatus, statusColorVar, type Status } from '../../utils/status';
import { StatusShapeIcon } from '../ui/Status';
import { TOOLTIP_STYLE } from '../../utils/chartConfig';
import ChartDataTable from '../ui/ChartDataTable';

interface Props {
  metrics: Metric[];
}

interface DonutSlice { status: Status; name: string; value: number; color: string }

const ORDER: Status[] = ['down', 'warning', 'unknown', 'maintenance', 'up'];

function buildDonutData(metrics: Metric[]): DonutSlice[] {
  const byDevice = new Map<number, Status>();
  for (const m of metrics) byDevice.set(m.deviceId, normalizeStatus(m.status));

  const counts = new Map<Status, number>();
  for (const status of byDevice.values()) counts.set(status, (counts.get(status) ?? 0) + 1);

  return ORDER.filter((status) => (counts.get(status) ?? 0) > 0).map((status) => ({
    status,
    name: STATUS_META[status].label,
    value: counts.get(status) ?? 0,
    color: statusColorVar(status),
  }));
}

function StatusDistributionInner({ metrics }: Props) {
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);
  const donutData = useMemo(() => buildDonutData(metrics), [metrics]);
  const donutTotal = useMemo(() => donutData.reduce((s, d) => s + d.value, 0), [donutData]);
  const active = activeIndex != null ? donutData[activeIndex] : undefined;

  return (
    <div className="bg-surface-1 rounded-lg p-4 border border-line flex flex-col group">
      <h3 className="text-sm font-sans font-semibold text-fg mb-3">Status distribution</h3>
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
                paddingAngle={2}
                dataKey="value"
                labelLine={false}
                animationBegin={120}
                animationDuration={700}
              >
                {donutData.map((entry, i) => (
                  <Cell
                    key={entry.name}
                    fill={entry.color}
                    stroke="var(--color-surface-1)"
                    strokeWidth={2}
                    onMouseEnter={() => setActiveIndex(i)}
                    onMouseLeave={() => setActiveIndex(undefined)}
                    style={{ cursor: 'pointer', opacity: activeIndex != null && activeIndex !== i ? 0.55 : 1 }}
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
                <span className={STATUS_META[d.status].text}><StatusShapeIcon shape={STATUS_META[d.status].shape} /></span>
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
