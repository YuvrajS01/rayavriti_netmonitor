import { memo, useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { TOOLTIP_STYLE, DEVICE_COLORS, AXIS_TICK_STYLE, LEGEND_STYLE, legendFormatter } from '../../utils/chartConfig';
import ChartDataTable from '../ui/ChartDataTable';

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
  const tableData = useMemo(() => {
    const columns = ['Time', ...devices];
    const rows = data.slice(-10).map((point) => [
      point.time,
      ...devices.map((d) => `${point[d] ?? '-'}ms`),
    ]);
    return { columns, rows };
  }, [data, devices]);

  return (
    <div
      className="xl:col-span-2 bg-surface-1 rounded-lg p-4 border border-line hover:border-line-strong transition-colors duration-200 cursor-pointer group"
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onExpand(); } }}
      onClick={onExpand}
    >
      <div className="flex justify-between items-center mb-3">
        <h3 className="text-sm font-sans font-semibold text-fg group-hover:text-fg transition-colors">Response Time per Device</h3>
        <span className="material-symbols-outlined text-fg-muted group-hover:text-fg text-sm transition-colors">open_in_full</span>
      </div>
      {devices.length === 0 ? (
        <p className="text-xs text-fg-muted text-center py-16">No device metrics yet</p>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={data} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
              <defs>{devices.map((dev, i) => <linearGradient id={`response-gradient-${i}`} key={dev} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor={DEVICE_COLORS[i % DEVICE_COLORS.length]} stopOpacity=".28"/><stop offset="1" stopColor={DEVICE_COLORS[i % DEVICE_COLORS.length]} stopOpacity="0"/></linearGradient>)}</defs>
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
                width={48}
              />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
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
                  activeDot={{ r: 4, strokeWidth: 0 }}
                  connectNulls
                  animationBegin={i * 120}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
          <div className="sr-only">
            <ChartDataTable title="Response Time Data" columns={tableData.columns} rows={tableData.rows} />
          </div>
        </>
      )}
    </div>
  );
}

export const ResponseTimeChart = memo(ResponseTimeChartInner);
