import { useState, useEffect, useCallback, useMemo } from 'react';
import { getAlerts, getAlertCounts, getGroupedAlerts, acknowledgeAlert, resolveAlert } from '../api/client';
import type { Alert, AlertCounts } from '../api/types';
import type { AlertGroup } from '../api/alerts';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';

function severityIcon(severity: string) {
  if (severity === 'critical') return 'dangerous';
  if (severity === 'warning') return 'warning';
  return 'info';
}

function severityColors(severity: string) {
  if (severity === 'critical') return { text: 'text-error', bg: 'bg-error/10', border: 'border-error' };
  if (severity === 'warning') return { text: 'text-warning', bg: 'bg-warning/10', border: 'border-warning' };
  return { text: 'text-primary', bg: 'bg-primary/10', border: 'border-primary' };
}

export default function Alerts() {

  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [grouped, setGrouped] = useState<AlertGroup[]>([]);
  const [counts, setCounts] = useState<AlertCounts>({ active: 0, acknowledged: 0, resolved: 0 });
  const [currentTab, setCurrentTab] = useState<'active' | 'acknowledged' | 'resolved'>('active');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'grouped' | 'list'>('grouped');

  const load = useCallback(async (tab: string = currentTab) => {
    setLoading(true);
    try {
      setError(null);
      const [alertsRes, countsRes, groupedRes] = await Promise.all([
        getAlerts(tab, 300),
        getAlertCounts(),
        getGroupedAlerts(tab),
      ]);
      setAlerts(alertsRes.data || []);
      setCounts(countsRes.data || { active: 0, acknowledged: 0, resolved: 0 });
      setGrouped(groupedRes.data || []);
    } catch {
      setError('Failed to load alerts. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [currentTab]);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { load(currentTab); }, [currentTab, load]);

  const handleAck = async (id: number) => {
    try {
      await acknowledgeAlert(id);
      load(currentTab);
    } catch {
      setError('Failed to acknowledge alert. Please try again.');
    }
  };

  const handleResolve = async (id: number) => {
    try {
      await resolveAlert(id);
      load(currentTab);
    } catch {
      setError('Failed to resolve alert. Please try again.');
    }
  };

  const tabs = [
    { key: 'active' as const, label: 'Active', count: counts.active },
    { key: 'acknowledged' as const, label: 'Acknowledged', count: counts.acknowledged },
    { key: 'resolved' as const, label: 'Resolved', count: counts.resolved },
  ];

  const critical = useMemo(() => alerts.filter((a) => a.severity === 'critical'), [alerts]);
  const warnings = useMemo(() => alerts.filter((a) => a.severity === 'warning'), [alerts]);
  const info = useMemo(() => alerts.filter((a) => a.severity === 'info'), [alerts]);

  return (
    <div>
      <SectionHeader
        title="Alerts"
        subtitle="Monitor and respond to network alerts across all devices."
      />

      {/* Status Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
        <StatCard label="Active" value={counts.active} color="text-error" icon="error" />
        <StatCard label="Acknowledged" value={counts.acknowledged} color="text-warning" icon="check_circle" />
        <StatCard label="Resolved" value={counts.resolved} color="text-primary" icon="done_all" />
      </div>

      {/* Tabs + View Toggle */}
      <div className="flex flex-wrap items-center justify-between gap-4 mb-6">
        <div className="flex flex-wrap gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`px-4 py-2 text-xs rounded-md border font-semibold uppercase tracking-wide transition-[border-color,color,background-color] ${
                currentTab === tab.key
                  ? 'border-primary/40 text-primary bg-primary/5'
                  : 'border-outline-variant/20 text-on-surface-variant hover:text-on-surface'
              }`}
            >
              {tab.label} ({tab.count})
            </button>
          ))}
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-on-surface-variant">Live alerts feed</span>
          {currentTab === 'active' && (
            <div className="flex gap-1 bg-surface-container-lowest rounded-lg p-0.5">
              <button
                onClick={() => setViewMode('grouped')}
                className={`px-3 py-1 text-xs rounded-md font-semibold uppercase tracking-wide transition-colors ${
                  viewMode === 'grouped' ? 'bg-primary/10 text-primary' : 'text-on-surface-variant hover:text-on-surface'
                }`}
              >
                Grouped
              </button>
              <button
                onClick={() => setViewMode('list')}
                className={`px-3 py-1 text-xs rounded-md font-semibold uppercase tracking-wide transition-colors ${
                  viewMode === 'list' ? 'bg-primary/10 text-primary' : 'text-on-surface-variant hover:text-on-surface'
                }`}
              >
                List
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Loading State */}
      {loading && <LoadingState message="Loading alerts..." />}

      {/* Error State */}
      {error && !loading && <ErrorState message={error} onRetry={() => load(currentTab)} />}

      {/* Grouped View */}
      {!loading && !error && currentTab === 'active' && viewMode === 'grouped' && (
        <div className="space-y-4">
          {grouped.length === 0 && (
            <div className="text-sm text-on-surface-variant text-center py-12">No alerts in this view</div>
          )}
          {grouped.map((group) => (
            <AlertGroupCard key={group.groupId} group={group} onAck={handleAck} onResolve={handleResolve} />
          ))}
        </div>
      )}

      {/* List View */}
      {!loading && !error && (currentTab !== 'active' || viewMode === 'list') && (
        <div className="space-y-4">
          {currentTab === 'active' && critical.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-error/20" />
                <span className="font-headline text-error font-semibold tracking-wide text-sm">Critical</span>
                <span className="h-px flex-1 bg-error/20" />
              </div>
              {critical.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && warnings.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-warning/20" />
                <span className="font-headline text-warning font-semibold tracking-wide text-sm">Warning</span>
                <span className="h-px flex-1 bg-warning/20" />
              </div>
              {warnings.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && info.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-primary/20" />
                <span className="font-headline text-primary font-semibold tracking-wide text-sm">Information</span>
                <span className="h-px flex-1 bg-primary/20" />
              </div>
              {info.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab !== 'active' && alerts.map((alert) => (
            <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />
          ))}

          {alerts.length === 0 && (
            <div className="text-sm text-on-surface-variant text-center py-12">No alerts in this view</div>
          )}
        </div>
      )}
    </div>
  );
}

function AlertGroupCard({ group, onAck, onResolve }: { group: AlertGroup; onAck: (id: number) => void; onResolve: (id: number) => void }) {
  const [expanded, setExpanded] = useState(false);
  const firstAlert = group.alerts[0];
  if (!firstAlert) return null;

  const sc = severityColors(firstAlert.severity);
  const severity = firstAlert.severity;

  return (
    <div className={`bg-surface-container-low rounded-lg border ${sc.border} overflow-hidden transition-colors`}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full p-5 flex items-center justify-between hover:bg-surface-container-low transition-colors"
      >
        <div className="flex items-center gap-4">
          <div className={`${sc.bg} p-3 rounded-lg`}>
            <span className={`material-symbols-outlined ${sc.text}`} style={{ fontVariationSettings: "'FILL' 1" }}>
              {severityIcon(severity)}
            </span>
          </div>
          <div className="text-left">
            <div className="flex items-center gap-3">
              <h3 className="font-headline font-semibold text-on-surface tracking-tight">{firstAlert.message.split(':')[0]}</h3>
              <span className={`text-xs ${sc.bg} ${sc.text} px-2 py-0.5 font-semibold rounded`}>
                {severity}
              </span>
            </div>
            <p className="text-sm text-on-surface-variant">{group.count} device{group.count !== 1 ? 's' : ''} affected</p>
          </div>
        </div>
        <span className="material-symbols-outlined text-on-surface-variant transition-transform" style={{ transform: expanded ? 'rotate(180deg)' : '' }}>
          expand_more
        </span>
      </button>
      {expanded && (
        <div className="border-t border-outline-variant/15 px-5 pb-4 space-y-3">
          {group.alerts.map((alert) => (
            <AlertItem key={alert.id} alert={alert} onAck={onAck} onResolve={onResolve} />
          ))}
        </div>
      )}
    </div>
  );
}

function AlertItem({ alert, onAck, onResolve }: { alert: Alert; onAck: (id: number) => void; onResolve: (id: number) => void }) {
  const deviceName = alert.deviceName || `Device ${alert.deviceId}`;

  return (
    <div className={`group bg-surface-container-low rounded-lg border ${severityColors(alert.severity).border} p-5 flex flex-col md:flex-row md:items-center justify-between gap-6 transition-[background-color,border-color] hover:bg-surface-container-low`}>
      <div className="flex items-start gap-4">
        <div className={`${severityColors(alert.severity).bg} p-3 rounded-lg`}>
          <span className={`material-symbols-outlined ${severityColors(alert.severity).text}`} style={{ fontVariationSettings: "'FILL' 1" }}>
            {severityIcon(alert.severity)}
          </span>
        </div>
        <div>
          <div className="flex items-center gap-3 mb-1 flex-wrap">
            <h3 className="font-headline font-semibold text-on-surface tracking-tight">{alert.message}</h3>
            <span className={`text-xs ${severityColors(alert.severity).bg} ${severityColors(alert.severity).text} px-2 py-0.5 font-semibold rounded`}>
              {alert.severity}
            </span>
          </div>
          <p className="text-sm text-on-surface-variant">{deviceName}</p>
          <span className="text-xs font-data text-on-surface-variant mt-2 block">
            {new Date(alert.createdAt).toLocaleString()} • {alert.status}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-4">
        {alert.status === 'active' && (
          <button onClick={() => onAck(alert.id)} className={`px-5 py-2.5 border ${severityColors(alert.severity).border} ${severityColors(alert.severity).text} font-headline font-semibold text-sm rounded-md hover:bg-surface-container-lowest transition-[background-color,color]`}>
            Acknowledge
          </button>
        )}
        {alert.status === 'acknowledged' && (
          <button onClick={() => onResolve(alert.id)} className="px-5 py-2.5 border border-primary text-primary font-headline font-semibold text-sm rounded-md hover:bg-primary hover:text-on-primary transition-[background-color,color]">
            Resolve
          </button>
        )}
        {alert.status === 'resolved' && (
          <span className="text-xs font-semibold tracking-wide text-on-surface-variant">Resolved</span>
        )}
      </div>
    </div>
  );
}
