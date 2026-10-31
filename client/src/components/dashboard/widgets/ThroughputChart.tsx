import { useMemo } from 'react';
import { Area, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import type { InterfaceHistoryPoint } from '../../../api/types';
import { AXIS_TICK_STYLE, DEVICE_COLORS, GRID_COLOR, LEGEND_STYLE, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE, legendFormatter } from '../../../utils/chartConfig';
import { useChartAnimation } from '../../../hooks/useReducedMotion';
import { formatBps } from '../../../utils/formatters';
import ChartDataTable from '../../ui/ChartDataTable';
import { formatBucket } from '../../charts/historyData';

interface Props {
  points: readonly InterfaceHistoryPoint[];
  height: number;
  title: string;
}

const SERIES = [
  { key: 'inBps', peak: 'inPeak', name: 'In', color: DEVICE_COLORS[0] },
  { key: 'outBps', peak: 'outPeak', name: 'Out', color: DEVICE_COLORS[1] },
] as const;

/**
 * Average in/out bit rate per bucket, with the bucket peak as a faint band
 * so short bursts stay visible at coarse buckets.
 */
export default function ThroughputChart({ points, height, title }: Props) {
  const anim = useChartAnimation();
  const data = useMemo(
    () => points.map((p) => ({ t: new Date(p.t).getTime(), inBps: p.inBps, outBps: p.outBps, inPeak: [p.inBps, p.maxInBps], outPeak: [p.outBps, p.maxOutBps] })),
    [points],
  );
  const span = data.length > 1 ? data[data.length - 1]!.t - data[0]!.t : 0;
  const table = useMemo(
    () => ({
      columns: ['Time', 'In', 'Out', 'Peak in', 'Peak out'],
      rows: points.slice(-10).map((p) => [new Date(p.t).toLocaleString(), formatBps(p.inBps), formatBps(p.outBps), formatBps(p.maxInBps), formatBps(p.maxOutBps)]),
    }),
    [points],
  );

  return (
    <>
      <ResponsiveContainer width="100%" height={height}>
        <ComposedChart data={data} margin={{ top: 8, right: 8, left: 4, bottom: 0 }}>
          <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="t" type="number" scale="time" domain={['dataMin', 'dataMax']} tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(t: number) => formatBucket(t, span)} minTickGap={32} />
          <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} width={72} tickFormatter={(v: number) => formatBps(v).replace(' ', '')} />
          <Tooltip
            contentStyle={TOOLTIP_STYLE}
            labelStyle={TOOLTIP_LABEL_STYLE}
            itemStyle={TOOLTIP_ITEM_STYLE}
            labelFormatter={(t) => new Date(Number(t)).toLocaleString()}
            formatter={(value: unknown, name: unknown) =>
              Array.isArray(value) ? [`peak ${formatBps(Number(value[1]))}`, `${String(name)}`] : [formatBps(Number(value)), String(name)]
            }
          />
          <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
          {SERIES.map((s) => (
            <Area key={s.peak} dataKey={s.peak} name={`${s.name} peak`} stroke="none" fill={s.color} fillOpacity={0.12} legendType="none" {...anim} />
          ))}
          {SERIES.map((s) => (
            <Line key={s.key} type="monotone" dataKey={s.key} name={s.name} stroke={s.color} strokeWidth={2} dot={false} activeDot={{ r: 4, strokeWidth: 0 }} {...anim} />
          ))}
        </ComposedChart>
      </ResponsiveContainer>
      <p className="mt-1 text-xs text-fg-muted">Lines are averages per bucket; the shading reaches each bucket’s peak.</p>
      <div className="sr-only">
        <ChartDataTable title={title} columns={table.columns} rows={table.rows} />
      </div>
    </>
  );
}
