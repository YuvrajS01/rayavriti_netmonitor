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
import EmptyState from '../components/ui/EmptyState';
import Button from '../components/ui/Button';
import { Tabs, TabPanel } from '../components/ui/Tabs';
import { StatusPill, StatusShapeIcon } from '../components/ui/Status';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { STATUS_META, lifecycleMeta, severityMeta, type Status } from '../utils/status';

const TAB_IDS = ['active', 'acknowledged', 'resolved'] as const;
type AlertTab = (typeof TAB_IDS)[number];

const EMPTY_COUNTS: AlertCounts = { active: 0, acknowledged: 0, resolved: 0 };
const EMPTY_ALERTS: Alert[] = [];
const EMPTY_GROUPS: AlertGroup[] = [];

const SEVERITY_SECTIONS: { severity: Alert['severity']; label: string; status: Status }[] = [
  { severity: 'critical', label: 'Critical', status: 'down' },
  { severity: 'warning', label: 'Warning', status: 'warning' },
  { severity: 'info', label: 'Information', status: 'info' },
];

export default function Alerts() {
  const queryClient = useQueryClient();
  const [currentTab, setCurrentTab] = useTabParam<AlertTab>(TAB_IDS);
  const tabsId = useTabsId();
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

  // Active alerts present when the page first loaded. A critical alert that
  // arrives later (via the live bridge) gets the one-off "new" pulse.
  const [baselineIds, setBaselineIds] = useState<Set<number> | null>(null);
  if (baselineIds === null && currentTab === 'active' && alertsQuery.isSuccess) {
    setBaselineIds(new Set(alerts.map((a) => a.id)));
  }
  const isNewCritical = (alert: Alert) =>
    alert.severity === 'critical' && alert.status === 'active' && baselineIds !== null && !baselineIds.has(alert.id);

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

  const bySeverity = useMemo(() => {
    const map: Record<Alert['severity'], Alert[]> = { critical: [], warning: [], info: [] };
    for (const alert of alerts) (map[alert.severity] ?? map.info).push(alert);
    return map;
  }, [alerts]);

  const itemProps = { onAck: handleAck, onResolve: handleResolve, isNew: isNewCritical };

  return (
    <div>
      <SectionHeader
        title="Alerts"
        subtitle="Monitor and respond to network alerts across all devices."
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <StatCard label="Active" value={counts.active} color={counts.active > 0 ? 'text-error' : 'text-fg'} icon="notifications_active" />
        <StatCard label="Acknowledged" value={counts.acknowledged} icon="check_circle" />
        <StatCard label="Resolved" value={counts.resolved} icon="done_all" />
      </div>

      {alerts.length > 0 && (
        <section aria-labelledby="severity-mix-heading" className="bg-surface-1 rounded-lg border border-line p-5 mb-6">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 id="severity-mix-heading" className="text-sm font-semibold">Severity mix</h2>
            <p className="text-xs text-fg-muted">{alerts.length} alerts in this view</p>
          </div>
          <div className="flex h-3 gap-0.5 rounded-full overflow-hidden mt-4 bg-inset" aria-hidden="true">
            {SEVERITY_SECTIONS.map(({ severity, status }) =>
              bySeverity[severity].length > 0 ? (
                <i key={severity} className={STATUS_META[status].bg} style={{ width: `${(bySeverity[severity].length / alerts.length) * 100}%` }} />
              ) : null,
            )}
          </div>
          <ul className="mt-3 flex flex-wrap gap-x-5 gap-y-1 text-xs">
            {SEVERITY_SECTIONS.map(({ severity, label, status }) => (
              <li key={severity} className="inline-flex items-center gap-1.5">
                <span className={STATUS_META[status].text}><StatusShapeIcon shape={STATUS_META[status].shape} /></span>
                <span className="text-fg-muted">{label}</span>
                <span className="font-semibold tabular-nums text-fg">{bySeverity[severity].length}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="flex flex-wrap items-end justify-between gap-4 mb-5">
        <Tabs
          label="Alert status"
          idBase={tabsId}
          value={currentTab}
          onChange={setCurrentTab}
          className="flex-1 min-w-0"
          items={[
            { id: 'active', label: 'Active', badge: counts.active },
            { id: 'acknowledged', label: 'Acknowledged', badge: counts.acknowledged },
            { id: 'resolved', label: 'Resolved', badge: counts.resolved },
          ]}
        />
        {currentTab === 'active' && (
          <div role="group" aria-label="View" className="flex gap-0.5 rounded-md border border-line bg-inset p-0.5">
            {(['grouped', 'list'] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                aria-pressed={viewMode === mode}
                onClick={() => setViewMode(mode)}
                className={`h-8 rounded-[6px] px-3 text-sm font-medium capitalize transition-colors ${
                  viewMode === mode ? 'bg-surface-3 text-fg shadow-e1' : 'text-fg-muted hover:text-fg'
                }`}
              >
                {mode}
              </button>
            ))}
          </div>
        )}
      </div>

      <TabPanel idBase={tabsId} id={currentTab} value={currentTab}>
        {loading && <LoadingState message="Loading alerts…" />}

        {error && !loading && <ErrorState message={error} onRetry={retry} />}

        {!loading && !error && currentTab === 'active' && viewMode === 'grouped' && (
          <div className="space-y-3">
            {grouped.length === 0 && <EmptyState icon="notifications_off" title="No active alerts" description="Everything is quiet right now." />}
            {grouped.map((group) => (
              <AlertGroupCard key={group.groupId} group={group} {...itemProps} />
            ))}
          </div>
        )}

        {!loading && !error && (currentTab !== 'active' || viewMode === 'list') && (
          <div className="space-y-3">
            {currentTab === 'active' &&
              SEVERITY_SECTIONS.map(({ severity, label, status }) =>
                bySeverity[severity].length > 0 ? (
                  <section key={severity} aria-label={`${label} alerts`} className="space-y-3">
                    <h2 className={`flex items-center gap-2 pt-3 text-sm font-semibold ${STATUS_META[status].text}`}>
                      <StatusShapeIcon shape={STATUS_META[status].shape} />
                      <span className="text-fg">{label}</span>
                      <span className="text-fg-muted font-normal tabular-nums">{bySeverity[severity].length}</span>
                      <span className="h-px flex-1 bg-line" />
                    </h2>
                    {bySeverity[severity].map((alert) => <AlertItem key={alert.id} alert={alert} {...itemProps} />)}
                  </section>
                ) : null,
              )}

            {currentTab !== 'active' && alerts.map((alert) => <AlertItem key={alert.id} alert={alert} {...itemProps} />)}

            {alerts.length === 0 && <EmptyState icon="notifications_off" title={`No ${currentTab} alerts`} />}
          </div>
        )}
      </TabPanel>
    </div>
  );
}

interface ItemHandlers {
  onAck: (id: number) => void;
  onResolve: (id: number) => void;
  isNew: (alert: Alert) => boolean;
}

function AlertGroupCard({ group, onAck, onResolve, isNew }: { group: AlertGroup } & ItemHandlers) {
  const [expanded, setExpanded] = useState(false);
  const firstAlert = group.alerts[0];
  if (!firstAlert) return null;

  const meta = severityMeta(firstAlert.severity);
  const panelId = `alert-group-${group.groupId}`;
  const hasNew = group.alerts.some(isNew);

  return (
    <div className={`bg-surface-1 rounded-lg border border-line border-l-4 ${meta.borderL} overflow-hidden ${hasNew ? 'alert-new-pulse' : ''}`}>
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-controls={panelId}
        className="w-full p-4 flex items-center justify-between gap-4 text-left hover:bg-surface-2 transition-colors"
      >
        <div className="flex items-center gap-4 min-w-0">
          <div className={`${meta.chip} p-2.5 rounded-md`}>
            <span className={`material-symbols-outlined ${meta.text}`} aria-hidden="true">{meta.icon}</span>
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-3 flex-wrap">
              <h3 className="font-semibold text-fg truncate">{firstAlert.message.split(':')[0]}</h3>
              <StatusPill status={meta} />
            </div>
            <p className="text-sm text-fg-muted">{group.count} device{group.count !== 1 ? 's' : ''} affected</p>
          </div>
        </div>
        <span className={`material-symbols-outlined text-fg-muted transition-transform ${expanded ? 'rotate-180' : ''}`} aria-hidden="true">
          expand_more
        </span>
      </button>
      {expanded && (
        <div id={panelId} className="border-t border-line p-4 space-y-3 bg-inset">
          {group.alerts.map((alert) => (
            <AlertItem key={alert.id} alert={alert} onAck={onAck} onResolve={onResolve} isNew={isNew} />
          ))}
        </div>
      )}
    </div>
  );
}

function AlertItem({ alert, onAck, onResolve, isNew }: { alert: Alert } & ItemHandlers) {
  const deviceName = alert.deviceName || `Device ${alert.deviceId}`;
  const meta = severityMeta(alert.severity);

  return (
    <article
      className={`bg-surface-1 rounded-lg border border-line border-l-4 ${meta.borderL} p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 ${isNew(alert) ? 'alert-new-pulse' : ''}`}
    >
      <div className="flex items-start gap-4 min-w-0">
        <div className={`${meta.chip} p-2.5 rounded-md shrink-0`}>
          <span className={`material-symbols-outlined ${meta.text}`} aria-hidden="true">{meta.icon}</span>
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-3 mb-1 flex-wrap">
            <h3 className="font-semibold text-fg">{alert.message}</h3>
            <StatusPill status={meta} />
            {isNew(alert) && <span className="text-xs font-medium text-fg-muted">New</span>}
          </div>
          <p className="text-sm text-fg-muted">{deviceName}</p>
          <p className="text-xs font-mono text-fg-subtle mt-1.5 flex flex-wrap items-center gap-2">
            <time dateTime={alert.createdAt}>{new Date(alert.createdAt).toLocaleString()}</time>
            <StatusPill status={lifecycleMeta(alert.status)} />
          </p>
        </div>
      </div>
      <div className="flex items-center gap-3 shrink-0">
        {alert.status === 'active' && (
          <Button variant="secondary" icon="check" onClick={() => onAck(alert.id)} aria-label={`Acknowledge alert: ${alert.message}`}>
            Acknowledge
          </Button>
        )}
        {alert.status === 'acknowledged' && (
          <Button variant="primary-outline" icon="done_all" onClick={() => onResolve(alert.id)} aria-label={`Resolve alert: ${alert.message}`}>
            Resolve
          </Button>
        )}
      </div>
    </article>
  );
}
