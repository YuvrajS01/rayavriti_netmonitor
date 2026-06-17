import { memo } from 'react';
import type { Alert } from '../../api/types';

interface Props {
  alerts: Alert[];
}

function ActiveAlertsListInner({ alerts }: Props) {
  return (
    <div className="bg-surface-container-high rounded-lg p-6 border border-outline-variant/20 flex flex-col">
      <div className="flex items-center gap-2 mb-6">
        <span className="material-symbols-outlined text-error text-xl">notifications_active</span>
        <h3 className="text-sm font-headline font-bold uppercase tracking-wide text-on-surface">Active Alerts</h3>
        {alerts.length > 0 && (
          <span className="ml-auto bg-error/20 text-error px-2 py-0.5 rounded-full text-[10px] font-bold" aria-label={`${alerts.length} active alerts`}>{alerts.length}</span>
        )}
      </div>
      <div className="space-y-3">
        {alerts.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 h-full">
            <span className="material-symbols-outlined text-4xl mb-2 text-primary">check_circle</span>
            <p className="text-xs text-on-surface-variant uppercase tracking-wide">All Systems Operational</p>
          </div>
        ) : (
          alerts.slice(0, 8).map((alert) => {
            const isCritical = alert.severity === 'critical';
            const isWarn = alert.severity === 'warning';
            const color = isCritical ? 'text-error' : isWarn ? 'text-warning' : 'text-primary';
            const bg = isCritical ? 'bg-error/10 border-error/30' : isWarn ? 'bg-warning/10 border-warning/30' : 'bg-primary/10 border-primary/30';
            const icon = isCritical ? 'error' : isWarn ? 'warning' : 'info';

            return (
              <div key={alert.id} className={`flex items-start gap-4 p-4 rounded-lg border ${bg} transition-[filter] hover:brightness-110`}>
                <span className={`material-symbols-outlined ${color} mt-0.5`}>{icon}</span>
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-1">
                    <span className="font-headline font-bold text-sm text-on-surface">{alert.deviceName || `Device ${alert.deviceId}`}</span>
                    <span className="text-[10px] font-mono text-on-surface-variant">{new Date(alert.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
                  </div>
                  <p className="text-xs text-on-surface-variant font-body">{alert.message}</p>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

export const ActiveAlertsList = memo(ActiveAlertsListInner);
