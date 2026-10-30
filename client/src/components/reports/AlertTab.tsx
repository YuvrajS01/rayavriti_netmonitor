import type { ReportAlert } from '../../api/types';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer } from 'recharts';
import { TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE } from '../../utils/chartConfig';
import { normalizeSeverity, severityMeta, type Severity } from '../../utils/status';
import { useChartAnimation } from '../../hooks/useReducedMotion';
import { StatusPill, StatusShapeIcon } from '../ui/Status';
import EmptyState from '../ui/EmptyState';

const SEVERITIES: Severity[] = ['critical', 'warning', 'info'];

export default function AlertTab({ alerts }: { alerts: ReportAlert[] }) {
  const anim = useChartAnimation();
  const bySev: Record<Severity, number> = { critical: 0, warning: 0, info: 0 };
  for (const a of alerts) bySev[normalizeSeverity(a.severity)] += 1;
  const donut = SEVERITIES.filter((sev) => bySev[sev] > 0).map((sev) => {
    const meta = severityMeta(sev);
    return { severity: sev, meta, name: meta.label, value: bySev[sev], color: `var(--color-${meta.token})` };
  });

  return (
    <div className="space-y-6 report-section">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <section aria-labelledby="alert-severity-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col items-center">
          <h3 id="alert-severity-heading" className="self-start text-sm font-semibold text-fg mb-4">Severity breakdown</h3>
          {alerts.length === 0 ? (
            <EmptyState icon="check_circle" title="No alerts in range" size="compact" />
          ) : (
            <>
              <ResponsiveContainer width="100%" height={180}>
                <PieChart>
                  <Pie data={donut} cx="50%" cy="50%" innerRadius={48} outerRadius={72} paddingAngle={2} dataKey="value" labelLine={false} {...anim}>
                    {donut.map(e => <Cell key={e.name} fill={e.color} stroke="var(--color-surface-1)" strokeWidth={2} />)}
                  </Pie>
                  <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} />
                </PieChart>
              </ResponsiveContainer>
              <ul className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
                {donut.map(d => (
                  <li key={d.name} className="flex items-center gap-1.5 text-xs">
                    <span className={d.meta.text}><StatusShapeIcon shape={d.meta.shape} /></span>
                    <span className="text-fg-muted">{d.name}</span>
                    <span className="font-semibold text-fg">{d.value}</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </section>

        <section aria-labelledby="alert-timeline-heading" className="lg:col-span-2 bg-surface-1 rounded-lg p-5 border border-line">
          <div className="flex items-center gap-2 mb-4">
            <h3 id="alert-timeline-heading" className="text-sm font-semibold text-fg">Alert timeline</h3>
            <span className="ml-auto text-xs text-fg-muted">{alerts.length} alerts</span>
          </div>
          {alerts.length === 0 ? (
            <p className="text-xs text-fg-muted text-center py-12">No alerts in selected range</p>
          ) : (
            <ul className="overflow-y-auto max-h-[420px] space-y-2 pr-1">
              {alerts.map((a) => {
                const meta = severityMeta(a.severity);
                return (
                  <li key={a.id} className={`flex items-start gap-3 rounded-md border border-line border-l-4 ${meta.borderL} bg-surface-2 p-3`}>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-2 mb-0.5">
                        <span className="font-semibold text-sm text-fg truncate">{a.deviceName || `Device ${a.deviceId}`}</span>
                        <time className="text-xs font-mono text-fg-muted shrink-0" dateTime={a.createdAt}>
                          {new Date(a.createdAt).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                        </time>
                      </div>
                      <p className="text-xs text-fg-muted truncate">{a.message}</p>
                    </div>
                    <StatusPill status={meta} />
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}
