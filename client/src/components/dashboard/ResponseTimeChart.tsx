import { memo, useMemo } from 'react';
import { AreaChart, Area, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { TOOLTIP_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_ITEM_STYLE, DEVICE_COLORS, AXIS_TICK_STYLE, GRID_COLOR, LEGEND_STYLE, legendFormatter } from '../../utils/chartConfig';
import { useChartAnimation } from '../../hooks/useReducedMotion';
import ChartDataTable from '../ui/ChartDataTable';
import IconButton from '../ui/IconButton';

interface MultiLinePoint {
  time: string;
  [deviceName: string]: string | number;
}

interface Props {
  data: MultiLinePoint[];
  devices: string[];
  onExpand: () => void;
}

function ResponseTimeChartInner({ data, devices, onExpand }: Props) {
  const anim = useChartAnimation();
  const tableData = useMemo(() => {
    const columns = ['Time', ...devices];
    const rows = data.slice(-10).map((point) => [
      point.time,
      ...devices.map((d) => `${point[d] ?? '-'}ms`),
    ]);
    return { columns, rows };
  }, [data, devices]);

  return (
    <section aria-labelledby="response-time-heading" className="xl:col-span-2 bg-surface-1 rounded-lg p-5 border border-line">
      <div className="flex justify-between items-center mb-3">
        <h3 id="response-time-heading" className="text-sm font-semibold text-fg">Response time per device</h3>
        <IconButton icon="open_in_full" label="Expand response time chart" size="sm" onClick={onExpand} />
      </div>
      {devices.length === 0 ? (
        <p className="text-xs text-fg-muted text-center py-16">No device metrics yet</p>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={data} margin={{ top: 4, right: 8, left: 4, bottom: 0 }}>
              <defs>{devices.map((dev, i) => <linearGradient id={`response-gradient-${i}`} key={dev} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor={DEVICE_COLORS[i % DEVICE_COLORS.length]} stopOpacity=".22"/><stop offset="1" stopColor={DEVICE_COLORS[i % DEVICE_COLORS.length]} stopOpacity="0"/></linearGradient>)}</defs>
              <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tick={AXIS_TICK_STYLE}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={AXIS_TICK_STYLE}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => `${v}ms`}
                width={56}
              />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                labelStyle={TOOLTIP_LABEL_STYLE}
                itemStyle={TOOLTIP_ITEM_STYLE}
                formatter={(value: unknown, name: unknown) => [`${Number(value ?? 0)}ms`, String(name)]}
              />
              <Legend
                wrapperStyle={LEGEND_STYLE}
                formatter={legendFormatter}
              />
              {devices.map((dev, i) => (
                <Area
                  key={dev}
                  type="monotone"
                  dataKey={dev}
                  stroke={DEVICE_COLORS[i % DEVICE_COLORS.length]}
                  strokeWidth={2}
                  fill={`url(#response-gradient-${i})`}
                  fillOpacity={i === 0 ? 1 : .35}
                  dot={data.length < 24 ? { r: 3, strokeWidth: 0, fill: DEVICE_COLORS[i % DEVICE_COLORS.length] } : false}
                  activeDot={{ r: 4, strokeWidth: 0 }}
                  connectNulls
                  {...anim}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
          <div className="sr-only">
            <ChartDataTable title="Response time data" columns={tableData.columns} rows={tableData.rows} />
          </div>
        </>
      )}
    </section>
  );
}

export const ResponseTimeChart = memo(ResponseTimeChartInner);
