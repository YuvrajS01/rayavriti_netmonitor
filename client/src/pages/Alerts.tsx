import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getAlerts, getAlertCounts, getGroupedAlerts, acknowledgeAlert, resolveAlert } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { Alert, AlertCounts } from '../api/types';
import type { AlertGroup } from '../api/alerts';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';
import Sparkline from '../components/ui/Sparkline';

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

const EMPTY_COUNTS: AlertCounts = { active: 0, acknowledged: 0, resolved: 0 };
const EMPTY_ALERTS: Alert[] = [];
const EMPTY_GROUPS: AlertGroup[] = [];

export default function Alerts() {
  const queryClient = useQueryClient();
  const [currentTab, setCurrentTab] = useState<'active' | 'acknowledged' | 'resolved'>('active');
  const [actionError, setActionError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'grouped' | 'list'>('grouped');

  // All three are invalidated by alert WebSocket events (useLiveQueryBridge),
  // so the list stays live without a full-page reload after each action.
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(currentTab, 300),
    queryFn: ({ signal }) => getAlerts(currentTab, 300, { signal }),
  });
  const countsQuery = useQuery({
    queryKey: queryKeys.alerts.counts(),
    queryFn: ({ signal }) => getAlertCounts({ signal }),
  });
  const groupedQuery = useQuery({
    queryKey: queryKeys.alerts.grouped(currentTab),
    queryFn: ({ signal }) => getGroupedAlerts(currentTab, { signal }),
  });

  const alerts = alertsQuery.data?.data || EMPTY_ALERTS;
  const grouped = groupedQuery.data?.data || EMPTY_GROUPS;
  const counts = countsQuery.data?.data || EMPTY_COUNTS;
  const queries = [alertsQuery, countsQuery, groupedQuery];
  const loading = queries.some((q) => q.isPending);
  const error = actionError ?? (queries.some((q) => q.isError) ? 'Failed to load alerts. Please try again.' : null);

  const retry = () => {
    setActionError(null);
    void queryClient.invalidateQueries({ queryKey: queryKeys.alerts.all });
  };

  const onActionSettled = () => queryClient.invalidateQueries({ queryKey: queryKeys.alerts.all });

  const ackMutation = useMutation({
    mutationFn: (id: number) => acknowledgeAlert(id),
    onMutate: () => setActionError(null),
    onError: () => setActionError('Failed to acknowledge alert. Please try again.'),
    onSettled: onActionSettled,
  });

  const resolveMutation = useMutation({
    mutationFn: (id: number) => resolveAlert(id),
    onMutate: () => setActionError(null),
    onError: () => setActionError('Failed to resolve alert. Please try again.'),
    onSettled: onActionSettled,
  });

  const handleAck = (id: number) => ackMutation.mutate(id);
  const handleResolve = (id: number) => resolveMutation.mutate(id);

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
        <StatCard label="Active" value={counts.active} color="text-error" icon="error" trend="down" sparklineData={[counts.active, critical.length, warnings.length, counts.active]} />
        <StatCard label="Acknowledged" value={counts.acknowledged} color="text-warning" icon="check_circle" trend="flat" sparklineData={[counts.acknowledged, warnings.length, counts.acknowledged]} />
        <StatCard label="Resolved" value={counts.resolved} color="text-primary" icon="done_all" trend="up" sparklineData={[counts.resolved, info.length, counts.resolved]} />
      </div>

      <div className="bg-surface-1 rounded-lg border border-line p-5 mb-6"><div className="flex flex-wrap items-center justify-between gap-4"><div><h2 className="font-sans text-sm font-semibold">Alert activity pulse</h2><p className="text-xs text-fg-muted mt-1">Severity distribution in the current view</p></div><Sparkline data={alerts.slice(0, 24).reverse().map((alert, index) => (alert.severity === 'critical' ? 9 : alert.severity === 'warning' ? 5 : 2) + index % 3)} width={220} height={52} color="var(--color-error)" /></div><div className="flex h-3 rounded-full overflow-hidden mt-5 bg-inset">{[[critical.length, 'bg-error'], [warnings.length, 'bg-warning'], [info.length, 'bg-info']].map(([value, color], index) => <i key={index} className={color as string} style={{ width: `${alerts.length ? Number(value) / alerts.length * 100 : 0}%` }} />)}</div></div>

      {/* Tabs + View Toggle */}
      <div className="flex flex-wrap items-center justify-between gap-4 mb-6">
        <div className="flex flex-wrap gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`px-4 py-2 text-xs rounded-md border font-semibold transition-[border-color,color,background-color] ${
                currentTab === tab.key
                  ? 'border-primary/40 text-primary bg-primary/5'
                  : 'border-line text-fg-muted hover:text-fg'
              }`}
            >
              {tab.label} ({tab.count})
            </button>
          ))}
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-fg-muted">Live alerts feed</span>
          {currentTab === 'active' && (
            <div className="flex gap-1 bg-inset rounded-lg p-0.5">
              <button
                onClick={() => setViewMode('grouped')}
                className={`px-3 py-1 text-xs rounded-md font-semibold transition-colors ${
                  viewMode === 'grouped'? 'bg-primary/10 text-primary': 'text-fg-muted hover:text-fg'
                }`}
              >
                Grouped
              </button>
              <button
                onClick={() => setViewMode('list')}
                className={`px-3 py-1 text-xs rounded-md font-semibold transition-colors ${
                  viewMode === 'list'? 'bg-primary/10 text-primary': 'text-fg-muted hover:text-fg'
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
      {error && !loading && <ErrorState message={error} onRetry={retry} />}

      {/* Grouped View */}
      {!loading && !error && currentTab === 'active' && viewMode === 'grouped' && (
        <div className="space-y-4">
          {grouped.length === 0 && (
            <div className="text-sm text-fg-muted text-center py-12">No alerts in this view</div>
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
                <span className="font-sans text-error font-semibold tracking-wide text-sm">Critical</span>
                <span className="h-px flex-1 bg-error/20" />
              </div>
              {critical.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && warnings.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-warning/20" />
                <span className="font-sans text-warning font-semibold tracking-wide text-sm">Warning</span>
                <span className="h-px flex-1 bg-warning/20" />
              </div>
              {warnings.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && info.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-primary/20" />
                <span className="font-sans text-primary font-semibold tracking-wide text-sm">Information</span>
                <span className="h-px flex-1 bg-primary/20" />
              </div>
              {info.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab !== 'active' && alerts.map((alert) => (
            <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />
          ))}

          {alerts.length === 0 && (
            <div className="text-sm text-fg-muted text-center py-12">No alerts in this view</div>
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
    <div className={`bg-surface-1 rounded-lg border ${sc.border} overflow-hidden transition-colors`}>
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full p-5 flex items-center justify-between hover:bg-surface-1 transition-colors"
      >
        <div className="flex items-center gap-4">
          <div className={`${sc.bg} p-3 rounded-lg`}>
            <span className={`material-symbols-outlined ${sc.text}`} style={{ fontVariationSettings: "'FILL' 1" }}>
              {severityIcon(severity)}
            </span>
          </div>
          <div className="text-left">
            <div className="flex items-center gap-3">
              <h3 className="font-sans font-semibold text-fg tracking-tight">{firstAlert.message.split(':')[0]}</h3>
              <span className={`text-xs ${sc.bg} ${sc.text} px-2 py-0.5 font-semibold rounded`}>
                {severity}
              </span>
            </div>
            <p className="text-sm text-fg-muted">{group.count} device{group.count !== 1 ? 's' : ''} affected</p>
          </div>
        </div>
        <span className="material-symbols-outlined text-fg-muted transition-transform" style={{ transform: expanded ? 'rotate(180deg)' : '' }}>
          expand_more
        </span>
      </button>
      {expanded && (
        <div className="border-t border-line px-5 pb-4 space-y-3">
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
    <div className={`group bg-surface-1 rounded-lg border ${severityColors(alert.severity).border} p-5 flex flex-col md:flex-row md:items-center justify-between gap-6 transition-[background-color,border-color] hover:bg-surface-1`}>
      <div className="flex items-start gap-4">
        <div className={`${severityColors(alert.severity).bg} p-3 rounded-lg`}>
          <span className={`material-symbols-outlined ${severityColors(alert.severity).text}`} style={{ fontVariationSettings: "'FILL' 1" }}>
            {severityIcon(alert.severity)}
          </span>
        </div>
        <div>
          <div className="flex items-center gap-3 mb-1 flex-wrap">
            <h3 className="font-sans font-semibold text-fg tracking-tight">{alert.message}</h3>
            <span className={`text-xs ${severityColors(alert.severity).bg} ${severityColors(alert.severity).text} px-2 py-0.5 font-semibold rounded`}>
              {alert.severity}
            </span>
          </div>
          <p className="text-sm text-fg-muted">{deviceName}</p>
          <span className="text-xs font-mono text-fg-muted mt-2 block">
            {new Date(alert.createdAt).toLocaleString()} • {alert.status}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-4">
        {alert.status === 'active' && (
          <button onClick={() => onAck(alert.id)} className={`px-5 py-2.5 border ${severityColors(alert.severity).border} ${severityColors(alert.severity).text} font-sans font-semibold text-sm rounded-md hover:bg-inset transition-[background-color,color]`}>
            Acknowledge
          </button>
        )}
        {alert.status === 'acknowledged' && (
          <button onClick={() => onResolve(alert.id)} className="px-5 py-2.5 border border-primary text-primary font-sans font-semibold text-sm rounded-md hover:bg-primary hover:text-on-primary transition-[background-color,color]">
            Resolve
          </button>
        )}
        {alert.status === 'resolved' && (
          <span className="text-xs font-semibold tracking-wide text-fg-muted">Resolved</span>
        )}
      </div>
    </div>
  );
}
