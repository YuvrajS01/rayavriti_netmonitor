import { useMemo } from 'react';
import { Area, CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import type { DeviceHistory } from '../../api/types';
import { AXIS_TICK_STYLE, DEVICE_COLORS, GRID_COLOR, LEGEND_STYLE, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE, legendFormatter } from '../../utils/chartConfig';
import { statusColorVar } from '../../utils/status';
import { useChartAnimation } from '../../hooks/useReducedMotion';
import ChartDataTable from '../ui/ChartDataTable';
import { formatBucket, mergeHistory } from './historyData';

interface Props {
  series: readonly DeviceHistory[];
  height?: number;
  /** Single device: draw the min–max band behind the average. */
  showRange?: boolean;
  title: string;
}

interface DotProps {
  cx?: number;
  cy?: number;
  payload?: Record<string, unknown>;
  dataKey?: unknown;
}

/** A square marker (the "down" shape) on buckets that had failed polls. */
function DownDot({ cx, cy, payload, dataKey }: DotProps) {
  const down = Number(payload?.[`${String(dataKey)}Down`] ?? 0);
  if (!down || cx == null || cy == null) return null;
  return <rect x={cx - 3.5} y={cy - 3.5} width={7} height={7} rx={1} fill={statusColorVar('down')} />;
}

/**
 * Bucketed response time (GET /metrics/history). One line per device in the
 * fixed categorical order (max 8); buckets with failed polls carry the down
 * square so outages aren't hidden by the average.
 */
export default function ResponseHistoryChart({ series, height = 260, showRange = false, title }: Props) {
  const anim = useChartAnimation();
  const shown = useMemo(() => series.slice(0, DEVICE_COLORS.length), [series]);
  const data = useMemo(() => mergeHistory(shown), [shown]);
  const span = data.length > 1 ? data[data.length - 1]!.t - data[0]!.t : 0;
  const names = useMemo(() => new Map(shown.map((s) => [`d${s.deviceId}`, s.deviceName])), [shown]);

  const table = useMemo(() => ({
    columns: ['Time', ...shown.map((s) => s.deviceName)],
    rows: data.slice(-10).map((row) => [
      new Date(row.t).toLocaleString(),
      ...shown.map((s) => {
        const v = row[`d${s.deviceId}`];
        return typeof v === 'number' ? `${Math.round(v)} ms` : '—';
      }),
    ]),
  }), [data, shown]);

  return (
    <>
      <ResponsiveContainer width="100%" height={height}>
        <ComposedChart data={data} margin={{ top: 8, right: 8, left: 4, bottom: 0 }}>
          <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="t"
            type="number"
            scale="time"
            domain={['dataMin', 'dataMax']}
            tick={AXIS_TICK_STYLE}
            tickLine={false}
            axisLine={false}
            tickFormatter={(t: number) => formatBucket(t, span)}
            minTickGap={32}
          />
          <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} width={56} tickFormatter={(v: number) => `${Math.round(v)}ms`} />
          <Tooltip
            contentStyle={TOOLTIP_STYLE}
            labelStyle={TOOLTIP_LABEL_STYLE}
            itemStyle={TOOLTIP_ITEM_STYLE}
            labelFormatter={(t) => new Date(Number(t)).toLocaleString()}
            formatter={(value: unknown, name: unknown) => {
              if (Array.isArray(value)) return [`${Math.round(Number(value[0]))}–${Math.round(Number(value[1]))} ms`, 'Min–max'];
              return [value == null ? '—' : `${Math.round(Number(value))} ms`, String(name)];
            }}
          />
          {shown.length > 1 && <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />}
          {showRange && shown[0] && (
            <Area
              dataKey={`d${shown[0].deviceId}Range`}
              name="Min–max"
              stroke="none"
              fill={DEVICE_COLORS[0]}
              fillOpacity={0.15}
              connectNulls={false}
              legendType="none"
              {...anim}
            />
          )}
          {shown.map((s, i) => (
            <Line
              key={s.deviceId}
              type="monotone"
              dataKey={`d${s.deviceId}`}
              name={names.get(`d${s.deviceId}`)}
              stroke={DEVICE_COLORS[i]}
              strokeWidth={2}
              dot={<DownDot />}
              activeDot={{ r: 4, strokeWidth: 0 }}
              connectNulls={false}
              {...anim}
            />
          ))}
        </ComposedChart>
      </ResponsiveContainer>
      <div className="sr-only">
        <ChartDataTable title={title} columns={table.columns} rows={table.rows} />
      </div>
    </>
  );
}
