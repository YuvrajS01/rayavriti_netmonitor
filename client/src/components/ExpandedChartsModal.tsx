import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import type { Metric } from '../api/types';

interface ExpandedChartsModalProps {
  metrics: Metric[];
  onClose: () => void;
}

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

// 6 distinct neon-ish colors for consistency
const DEVICE_COLORS = ['#d9fd3a', '#ff7351', '#6ee7f7', '#c084fc', '#4ade80', '#fb923c'];

export default function ExpandedChartsModal({ metrics, onClose }: ExpandedChartsModalProps) {
  // Group metrics by device
  const byDevice = new Map<string, Metric[]>();
  for (const m of metrics) {
    const key = m.device_name || `Device ${m.device_id}`;
    if (!byDevice.has(key)) byDevice.set(key, []);
    byDevice.get(key)!.push(m);
  }

  const devices = Array.from(byDevice.keys());

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md" onClick={onClose}>
      <div 
        className="bg-surface-container-low border border-outline-variant/30 rounded-xl w-full max-w-7xl h-[90vh] overflow-hidden shadow-2xl flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-center bg-surface-container-high">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">query_stats</span>
            <div>
              <h2 className="font-headline text-2xl font-black text-on-surface uppercase tracking-tight">Detailed Response Times</h2>
              <p className="text-on-surface-variant text-xs font-mono uppercase tracking-widest">Individual node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-colors">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        {/* Grid of Charts */}
        <div className="p-6 overflow-y-auto flex-1 bg-surface-container-lowest">
          {devices.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full opacity-50">
              <span className="material-symbols-outlined text-6xl mb-4">monitoring</span>
              <p className="text-sm font-headline uppercase tracking-widest text-on-surface-variant">No telemetry data available</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {devices.map((dev, i) => {
                const devMetrics = byDevice.get(dev) ?? [];
                // Take up to 30 recent readings, reverse so oldest is first (left to right)
                const chartData = devMetrics.slice(0, 30).reverse().map(m => ({
                  time: new Date(m.timestamp || m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
                  response: m.response_time ?? 0,
                  status: m.status
                }));
                const latest = chartData[chartData.length - 1];
                const color = DEVICE_COLORS[i % DEVICE_COLORS.length];

                return (
                  <div key={dev} className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 shadow-lg flex flex-col">
                    <div className="flex justify-between items-start mb-4">
                      <div>
                        <h3 className="font-headline text-lg font-bold text-on-surface truncate pr-2">{dev}</h3>
                        <p className="text-[10px] text-on-surface-variant uppercase tracking-widest font-mono mt-1">
                          Latest: <span style={{ color }}>{latest?.response ?? '-'}ms</span>
                        </p>
                      </div>
                      <div className={`px-2 py-0.5 rounded-full text-[9px] font-bold uppercase tracking-widest border
                        ${latest?.status === 'down' ? 'border-error text-error bg-error/10' : 
                          (latest?.status === 'warning' || latest?.status === 'degraded' ? 'border-amber-400 text-amber-400 bg-amber-400/10' : 
                          'border-primary text-primary bg-primary/10')}`}>
                        {latest?.status || 'Unknown'}
                      </div>
                    </div>
                    
                    <div className="h-48 w-full mt-auto">
                      <ResponsiveContainer width="100%" height="100%">
                        <LineChart data={chartData} margin={{ top: 5, right: 5, left: -25, bottom: 0 }}>
                          <XAxis 
                            dataKey="time" 
                            tick={{ fill: '#8a8a78', fontSize: 9 }} 
                            tickLine={false} 
                            axisLine={false}
                            minTickGap={20}
                          />
                          <YAxis 
                            tick={{ fill: '#8a8a78', fontSize: 9 }} 
                            tickLine={false} 
                            axisLine={false} 
                            tickFormatter={(v) => `${v}ms`}
                          />
                          <Tooltip 
                            contentStyle={TOOLTIP_STYLE}
                            formatter={(value: unknown) => [`${Number(value ?? 0)}ms`, 'Response']}
                            labelStyle={{ color: '#8a8a78', marginBottom: '4px', fontSize: '10px' }}
                          />
                          <Line 
                            type="monotone" 
                            dataKey="response" 
                            stroke={color} 
                            strokeWidth={2} 
                            dot={false} 
                            activeDot={{ r: 4, strokeWidth: 0 }} 
                            isAnimationActive={false}
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
