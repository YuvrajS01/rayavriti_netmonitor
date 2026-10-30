import { useMemo } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, CartesianGrid } from 'recharts';
import type { Metric } from '../api/types';
import { AXIS_TICK_STYLE, GRID_COLOR, SERIES_COLORS, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE } from '../utils/chartConfig';
import { useChartAnimation } from '../hooks/useReducedMotion';
import Dialog from './ui/Dialog';
import EmptyState from './ui/EmptyState';
import { StatusPill } from './ui/Status';

interface ExpandedChartsModalProps {
  metrics: Metric[];
  onClose: () => void;
}

/**
 * Small multiples: one chart per device. Each panel is titled with its
 * device, so every line uses the same series colour (no colour cycling).
 */
export default function ExpandedChartsModal({ metrics, onClose }: ExpandedChartsModalProps) {
  const anim = useChartAnimation();
  const byDevice = useMemo(() => {
    const map = new Map<string, Metric[]>();
    for (const m of metrics) {
      const key = m.deviceName || `Device ${m.deviceId}`;
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(m);
    }
    return map;
  }, [metrics]);

  const devices = useMemo(() => Array.from(byDevice.keys()), [byDevice]);

  return (
    <Dialog
      open
      onClose={onClose}
      size="2xl"
      title="Detailed response times"
      description="Per-device response time, last 30 polls"
      bodyClassName="p-6 bg-inset"
    >
      {devices.length === 0 ? (
        <EmptyState icon="monitoring" title="No telemetry data available" />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {devices.map((dev) => {
            const devMetrics = byDevice.get(dev) ?? [];
            const chartData = devMetrics.slice(0, 30).reverse().map((m) => ({
              time: new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
              response: m.responseTime ?? 0,
              status: m.status,
            }));
            const latest = chartData[chartData.length - 1];

            return (
              <section key={dev} aria-label={dev} className="bg-surface-1 rounded-lg p-4 border border-line flex flex-col">
                <div className="flex justify-between items-start gap-2 mb-3">
                  <div className="min-w-0">
                    <h3 className="text-sm font-semibold text-fg truncate">{dev}</h3>
                    <p className="text-xs text-fg-muted mt-0.5">
                      Latest <span className="font-mono text-fg">{latest?.response ?? '—'}ms</span>
                    </p>
                  </div>
                  <StatusPill status={latest?.status ?? 'unknown'} />
                </div>

                <div className="h-44 w-full mt-auto">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={chartData} margin={{ top: 5, right: 5, left: 0, bottom: 0 }}>
                      <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                      <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} minTickGap={24} />
                      <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                      <Tooltip
                        contentStyle={TOOLTIP_STYLE}
                        labelStyle={TOOLTIP_LABEL_STYLE}
                        itemStyle={TOOLTIP_ITEM_STYLE}
                        formatter={(value: unknown) => [`${Number(value ?? 0)}ms`, 'Response']}
                      />
                      <Line
                        type="monotone"
                        dataKey="response"
                        stroke={SERIES_COLORS[0]}
                        strokeWidth={2}
                        dot={false}
                        activeDot={{ r: 4, strokeWidth: 0 }}
                        {...anim}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </section>
            );
          })}
        </div>
      )}
    </Dialog>
  );
}
