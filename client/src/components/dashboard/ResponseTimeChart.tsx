import { memo, useMemo } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
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
      className="xl:col-span-2 bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 hover:border-primary/50 transition-[border-color] cursor-pointer group"
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onExpand(); } }}
      onClick={onExpand}
    >
      <div className="flex justify-between items-center mb-3">
        <h3 className="text-sm font-headline font-bold uppercase tracking-wide group-hover:text-primary transition-colors">Response Time per Device</h3>
        <span className="material-symbols-outlined text-on-surface-variant group-hover:text-primary text-sm transition-colors">open_in_full</span>
      </div>
      {devices.length === 0 ? (
        <p className="text-xs text-on-surface-variant text-center py-16">No device metrics yet</p>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={240}>
            <LineChart data={data} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
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
                <Line
                  key={dev}
                  type="monotone"
                  dataKey={dev}
                  stroke={DEVICE_COLORS[i % DEVICE_COLORS.length]}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4 }}
                  connectNulls
                />
              ))}
            </LineChart>
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
