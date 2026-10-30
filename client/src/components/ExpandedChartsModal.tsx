import { useEffect, useRef, useMemo } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import type { Metric } from '../api/types';
import { TOOLTIP_STYLE, DEVICE_COLORS } from '../utils/chartConfig';

interface ExpandedChartsModalProps {
  metrics: Metric[];
  onClose: () => void;
}

export default function ExpandedChartsModal({ metrics, onClose }: ExpandedChartsModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);

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

  useEffect(() => {
    previousFocus.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
        return;
      }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocus.current?.focus();
    };
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/80" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Detailed response times"
        tabIndex={-1}
        className="bg-surface-1 border border-line rounded-lg w-full max-w-7xl h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        <div className="p-6 border-b border-line flex justify-between items-center bg-surface-1 shrink-0">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">query_stats</span>
            <div>
              <h2 className="font-sans text-2xl font-semibold text-fg">Detailed response times</h2>
              <p className="text-fg-muted text-xs font-mono">Individual node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-inset rounded-full transition-colors" aria-label="Close dialog">
            <span className="material-symbols-outlined text-fg-subtle hover:text-fg">close</span>
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 bg-inset">
          {devices.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full opacity-50">
              <span className="material-symbols-outlined text-6xl mb-4">monitoring</span>
              <p className="text-sm font-sans text-fg-muted">No telemetry data available</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {devices.map((dev, i) => {
                const devMetrics = byDevice.get(dev) ?? [];
                const chartData = devMetrics.slice(0, 30).reverse().map(m => ({
                  time: new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
                  response: m.responseTime ?? 0,
                  status: m.status
                }));
                const latest = chartData[chartData.length - 1];
                const color = DEVICE_COLORS[i % DEVICE_COLORS.length];

                return (
                  <div key={dev} className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
                    <div className="flex justify-between items-start mb-4">
                      <div>
                        <h3 className="font-sans text-lg font-semibold text-fg truncate pr-2">{dev}</h3>
                        <p className="text-xs text-fg-muted font-mono mt-1">
                          Latest: <span style={{ color }}>{latest?.response ?? '-'}ms</span>
                        </p>
                      </div>
                      <div className={`capitalize px-2 py-0.5 rounded-full text-xs font-semibold border
                        ${latest?.status === 'down'? 'border-error text-error bg-error/10':
                          (latest?.status === 'warning'|| latest?.status === 'degraded'? 'border-warning text-warning bg-warning/10':
                          'border-primary text-primary bg-primary/10')}`}>
                        {latest?.status || 'Unknown'}
                      </div>
                    </div>

                    <div className="h-48 w-full mt-auto">
                      <ResponsiveContainer width="100%" height="100%">
                        <LineChart data={chartData} margin={{ top: 5, right: 5, left: -25, bottom: 0 }}>
                          <XAxis
                            dataKey="time"
                            tick={{ fill: '#77766d', fontSize: 9 }}
                            tickLine={false}
                            axisLine={false}
                            minTickGap={20}
                          />
                          <YAxis
                            tick={{ fill: '#77766d', fontSize: 9 }}
                            tickLine={false}
                            axisLine={false}
                            tickFormatter={(v) => `${v}ms`}
                          />
                          <Tooltip
                            contentStyle={TOOLTIP_STYLE}
                            formatter={(value: unknown) => [`${Number(value ?? 0)}ms`, 'Response']}
                            labelStyle={{ color: '#77766d', marginBottom: '4px', fontSize: '10px' }}
                          />
                          <Line
                            type="monotone"
                            dataKey="response"
                            stroke={color}
                            strokeWidth={2}
                            dot={false}
                            activeDot={{ r: 4, strokeWidth: 0 }}
                          />
                        </LineChart>
                      </ResponsiveContainer>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
