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
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Detailed response times"
        tabIndex={-1}
        className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-7xl h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-center bg-surface-container-high">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">query_stats</span>
            <div>
              <h2 className="font-headline text-2xl font-bold text-on-surface uppercase tracking-tight">Detailed Response Times</h2>
              <p className="text-on-surface-variant text-xs font-mono uppercase tracking-wide">Individual node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-[background-color]" aria-label="Close dialog">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 bg-surface-container-lowest">
          {devices.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full opacity-50">
              <span className="material-symbols-outlined text-6xl mb-4">monitoring</span>
              <p className="text-sm font-headline uppercase tracking-wide text-on-surface-variant">No telemetry data available</p>
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
                  <div key={dev} className="bg-surface-container-high rounded-lg p-5 border border-outline-variant/20 flex flex-col">
                    <div className="flex justify-between items-start mb-4">
                      <div>
                        <h3 className="font-headline text-lg font-bold text-on-surface truncate pr-2">{dev}</h3>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-wide font-mono mt-1">
                          Latest: <span style={{ color }}>{latest?.response ?? '-'}ms</span>
                        </p>
                      </div>
                      <div className={`px-2 py-0.5 rounded-full text-[9px] font-bold uppercase tracking-wide border
                        ${latest?.status === 'down' ? 'border-error text-error bg-error/10' :
                          (latest?.status === 'warning' || latest?.status === 'degraded' ? 'border-warning text-warning bg-warning/10' :
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
