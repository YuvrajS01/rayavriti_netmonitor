import type { ReportAlert } from '../../api/types';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts';

const TT = { background: 'var(--color-surface-container)', border: '1px solid var(--color-outline-variant)', borderRadius: '8px', fontSize: '12px', color: 'var(--color-on-surface)' };
const SEV_COLORS: Record<string, string> = { critical: '#ff7351', warning: '#e5a910', info: '#6bb8c9' };
const SEV_ICONS: Record<string, string> = { critical: 'error', warning: 'warning', info: 'info' };

export default function AlertTab({ alerts }: { alerts: ReportAlert[] }) {
  const bySev: Record<string, number> = { critical: 0, warning: 0, info: 0 };
  for (const a of alerts) bySev[a.severity] = (bySev[a.severity] || 0) + 1;
  const donut = Object.entries(bySev).filter(([, v]) => v > 0).map(([name, value]) => ({ name, value, color: SEV_COLORS[name] || '#6b7280' }));

  return (
    <div className="space-y-6 report-section">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Severity donut */}
        <div className="bg-surface-container-high rounded-lg p-6 border border-outline-variant/20 flex flex-col items-center">
          <h4 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant mb-4">Severity Breakdown</h4>
          {alerts.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 opacity-50">
              <span className="material-symbols-outlined text-4xl mb-2 text-primary">check_circle</span>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">No alerts in range</p>
            </div>
          ) : (
            <>
              <ResponsiveContainer width="100%" height={180}>
                <PieChart>
                  <Pie data={donut} cx="50%" cy="50%" innerRadius={48} outerRadius={72} paddingAngle={3} dataKey="value" labelLine={false}>
                    {donut.map(e => <Cell key={e.name} fill={e.color} stroke="transparent" />)}
                  </Pie>
                  <Tooltip contentStyle={TT} />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
                {donut.map(d => (
                  <div key={d.name} className="flex items-center gap-1.5 text-xs">
                    <span className="w-2.5 h-2.5 rounded-full" style={{ background: d.color }} />
                    <span className="text-on-surface-variant capitalize">{d.name}</span>
                    <span className="font-bold text-on-surface">{d.value}</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>

        {/* Alert table */}
        <div className="lg:col-span-2 bg-surface-container-high rounded-lg p-6 border border-outline-variant/20">
          <div className="flex items-center gap-2 mb-4">
            <span className="material-symbols-outlined text-error text-lg">notifications_active</span>
            <h4 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant">Alert Timeline</h4>
            <span className="ml-auto text-[10px] text-on-surface-variant">{alerts.length} alerts</span>
          </div>
          {alerts.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-12">No alerts in selected range</p>
          ) : (
            <div className="overflow-y-auto max-h-[420px] space-y-2 pr-1">
              {alerts.map((a) => {
                const color = SEV_COLORS[a.severity] || '#6b7280';
                const bg = a.severity === 'critical' ? 'bg-error/10 border-error/25' : a.severity === 'warning' ? 'bg-warning/10 border-warning/25' : 'bg-info/10 border-info/25';
                return (
                  <div key={a.id} className={`flex items-start gap-3 p-3 rounded-lg border ${bg} transition-[filter] hover:bg-surface-container-high`}>
                    <span className="material-symbols-outlined mt-0.5 text-sm" style={{ color }}>{SEV_ICONS[a.severity] || 'info'}</span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-2 mb-0.5">
                        <span className="font-headline font-bold text-xs text-on-surface truncate">{a.deviceName || `Device ${a.deviceId}`}</span>
                        <span className="text-[10px] font-mono text-on-surface-variant shrink-0">{new Date(a.createdAt).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                      </div>
                      <p className="text-xs text-on-surface-variant truncate">{a.message}</p>
                    </div>
                    <span className={`shrink-0 px-2 py-0.5 rounded-full text-[9px] font-bold uppercase border`} style={{ color, borderColor: `${color}40` }}>{a.severity}</span>
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
