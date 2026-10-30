import { memo, useState } from 'react';
import type { Alert } from '../../api/types';
import { severityMeta } from '../../utils/status';
import { StatusPill } from '../ui/Status';
import EmptyState from '../ui/EmptyState';

interface Props {
  alerts: Alert[];
}

function ActiveAlertsListInner({ alerts }: Props) {
  // Alerts present on first render are "known"; only critical alerts that
  // arrive afterwards get the (finite) new-alert pulse.
  const [initialIds] = useState(() => new Set(alerts.map((a) => a.id)));

  return (
    <section aria-labelledby="active-alerts-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
      <div className="flex items-center gap-2 mb-4">
        <h3 id="active-alerts-heading" className="text-sm font-semibold text-fg">Active alerts</h3>
        {alerts.length > 0 && (
          <span className="ml-auto rounded-full bg-surface-3 px-2 py-0.5 text-xs font-semibold tabular-nums text-fg" aria-label={`${alerts.length} active alerts`}>
            {alerts.length}
          </span>
        )}
      </div>
      {alerts.length === 0 ? (
        <EmptyState icon="check_circle" title="All systems operational" description="No active alerts." size="compact" />
      ) : (
        <ul className="space-y-2">
          {alerts.slice(0, 8).map((alert) => {
            const meta = severityMeta(alert.severity);
            const isNewCritical = meta.status === 'down' && !initialIds.has(alert.id);
            return (
              <li
                key={alert.id}
                className={`flex items-start gap-3 rounded-md border border-line border-l-4 ${meta.borderL} bg-surface-2 p-3 ${isNewCritical ? 'alert-new-pulse' : ''}`}
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2 mb-1">
                    <span className="truncate text-sm font-medium text-fg">{alert.deviceName || `Device ${alert.deviceId}`}</span>
                    <time className="shrink-0 text-xs font-mono text-fg-muted" dateTime={alert.createdAt}>
                      {new Date(alert.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </time>
                  </div>
                  <div className="flex items-start justify-between gap-2">
                    <p className="text-xs text-fg-muted">{alert.message}</p>
                    <StatusPill status={meta} />
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

export const ActiveAlertsList = memo(ActiveAlertsListInner);
