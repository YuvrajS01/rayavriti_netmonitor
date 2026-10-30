import { useState, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createIncident, getIncidentStats, getIncidents, runIncidentAction, type Incident, type IncidentAction, type IncidentStats } from '../api/incidents';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';
import { lifecycleMeta, severityMeta, statusColorVar } from '../utils/status';
import Dialog from '../components/ui/Dialog';
import { StatusPill } from '../components/ui/Status';
import GanttTimeline from '../components/charts/GanttTimeline';

const EMPTY_INCIDENTS: Incident[] = [];
const SEVERITIES = ['critical', 'major', 'minor', 'info'] as const;
const STATUSES = ['open', 'acknowledged', 'resolved', 'closed'] as const;

function formatDuration(sec: number | null): string {
  if (sec == null) return '-';
  if (sec < 60) return `${Math.round(sec)}s`;
  if (sec < 3600) return `${Math.round(sec / 60)}m`;
  return `${(sec / 3600).toFixed(1)}h`;
}

function timeAgo(ts: string | null): string {
  if (!ts) return '-';
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export default function Incidents() {
  const navigate = useNavigate();
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [severityFilter, setSeverityFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ title: '', description: '', severity: 'minor', impact_description: '' });

  // Invalidated on alert WebSocket events by useLiveQueryBridge.
  const incidentsQuery = useQuery({
    queryKey: queryKeys.incidents.list(),
    queryFn: ({ signal }) => getIncidents({ signal }),
  });
  const statsQuery = useQuery({
    queryKey: queryKeys.incidents.stats(),
    queryFn: ({ signal }) => getIncidentStats({ signal }),
  });
  const loading = incidentsQuery.isPending;
  const incidents = incidentsQuery.data?.data || EMPTY_INCIDENTS;
  const stats: IncidentStats | null = statsQuery.data?.data ?? null;

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.incidents.all });

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return incidents.filter((i) => {
      const matchSearch = !needle || i.title.toLowerCase().includes(needle) || i.description?.toLowerCase().includes(needle);
      const matchSev = severityFilter === 'all' || i.severity === severityFilter;
      const matchStatus = statusFilter === 'all' || i.status === statusFilter;
      return matchSearch && matchSev && matchStatus;
    });
  }, [incidents, search, severityFilter, statusFilter]);

  const createMutation = useMutation({
    mutationFn: (body: typeof createForm) => createIncident(body),
    onSuccess: () => {
      addToast('Incident created', 'success');
      setShowCreate(false);
      setCreateForm({ title: '', description: '', severity: 'minor', impact_description: '' });
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Create failed', 'error'),
  });
  const submitting = createMutation.isPending;

  const actionMutation = useMutation({
    mutationFn: ({ id, action, body }: { id: number; action: IncidentAction; body?: Record<string, unknown>; done: string }) =>
      runIncidentAction(id, action, body),
    onSuccess: (_res, { done }) => {
      addToast(done, 'success');
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Failed', 'error'),
  });

  const handleCreate = () => createMutation.mutate(createForm);
  const handleAcknowledge = (id: number) => actionMutation.mutate({ id, action: 'acknowledge', done: 'Incident acknowledged' });
  const handleResolve = (id: number) => actionMutation.mutate({ id, action: 'resolve', body: { resolution: 'Resolved via dashboard' }, done: 'Incident resolved' });
  const handleClose = (id: number) => actionMutation.mutate({ id, action: 'close', done: 'Incident closed' });

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Incidents"
        subtitle="Track, assign, and resolve network incidents with SLA tracking."
        action={<Button icon="add" onClick={() => setShowCreate(true)}>New incident</Button>}
      />

      {stats && (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
          <StatCard label="Open" value={stats.open} icon="radio_button_checked" color={stats.open > 0 ? 'text-error' : undefined} />
          <StatCard label="Acknowledged" value={stats.acknowledged} icon="visibility" />
          <StatCard label="Resolved" value={stats.resolved} icon="check_circle" />
          <StatCard label="SLA breached" value={stats.sla_breached} icon="gpp_bad" color={stats.sla_breached > 0 ? 'text-error' : undefined} />
          <StatCard label="Avg duration" value={formatDuration(stats.avg_duration_seconds)} icon="schedule" />
        </div>
      )}

      <GanttTimeline title="Recent incident windows" items={incidents.map((incident) => ({ id: incident.id, label: incident.title, start: incident.started_at, end: incident.resolved_at || incident.closed_at, color: statusColorVar(severityMeta(incident.severity).status) }))} />

      <Card variant="low" className="overflow-hidden">
        <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
          <h2 className="text-base font-semibold">All incidents <span className="text-sm font-normal text-fg-muted tabular-nums">({filtered.length})</span></h2>
          <div className="flex gap-3 flex-wrap">
            <label>
              <span className="sr-only">Filter by severity</span>
              <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className={INPUT_CLASS}>
                <option value="all">All severities</option>
                {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
              </select>
            </label>
            <label>
              <span className="sr-only">Filter by status</span>
              <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className={INPUT_CLASS}>
                <option value="all">All statuses</option>
                {STATUSES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
              </select>
            </label>
            <label className="relative md:w-56">
              <span className="sr-only">Search incidents</span>
              <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
              <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search incidents" className={`${INPUT_CLASS} pl-10`} />
            </label>
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading…</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="crisis_alert" title="No incidents" description="All clear — no active incidents on the network." />
        ) : (
          <ul className="divide-y divide-line">
            {filtered.map((inc) => {
              const sev = severityMeta(inc.severity);
              return (
                <li key={inc.id} className="p-4 hover:bg-surface-2 transition-colors cursor-pointer" onClick={() => navigate(`/incidents/${inc.id}`)}>
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className={`w-10 h-10 rounded-md flex items-center justify-center flex-shrink-0 ${sev.chip}`}>
                        <span className={`material-symbols-outlined text-xl ${sev.text}`} aria-hidden="true">{sev.icon}</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="font-semibold truncate">
                            <Link to={`/incidents/${inc.id}`} onClick={(e) => e.stopPropagation()} className="hover:underline">{inc.title}</Link>
                          </h3>
                          {inc.sla_breached && <StatusPill status="down">SLA breach</StatusPill>}
                        </div>
                        <p className="text-xs text-fg-muted mt-0.5">
                          {inc.source || 'manual'} · #{inc.id} · Started {timeAgo(inc.started_at)}
                        </p>
                        {inc.description && <p className="text-sm text-fg-muted mt-2 line-clamp-2">{inc.description}</p>}
                      </div>
                    </div>
                    <dl className="grid grid-cols-2 sm:grid-cols-4 gap-4 lg:max-w-xl flex-shrink-0">
                      <div>
                        <dt className="text-xs text-fg-muted">Severity</dt>
                        <dd><StatusPill status={sev} className="mt-0.5" /></dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Status</dt>
                        <dd><StatusPill status={lifecycleMeta(inc.status)} className="mt-0.5" /></dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Devices</dt>
                        <dd className="text-sm font-medium tabular-nums">{inc.affected_device_count}</dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Duration</dt>
                        <dd className="text-sm font-medium tabular-nums">{formatDuration(inc.duration_seconds)}</dd>
                      </div>
                    </dl>
                  </div>
                  {inc.status !== 'closed' && (
                    <div className="flex gap-2 mt-3 pt-3 border-t border-line">
                      {inc.status === 'open' && (
                        <Button size="sm" variant="secondary" icon="visibility" onClick={(e) => { e.stopPropagation(); handleAcknowledge(inc.id); }}>Acknowledge</Button>
                      )}
                      {(inc.status === 'open' || inc.status === 'acknowledged') && (
                        <Button size="sm" variant="secondary" icon="check_circle" onClick={(e) => { e.stopPropagation(); handleResolve(inc.id); }}>Resolve</Button>
                      )}
                      {inc.status === 'resolved' && (
                        <Button size="sm" variant="secondary" icon="archive" onClick={(e) => { e.stopPropagation(); handleClose(inc.id); }}>Close</Button>
                      )}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </Card>

      <Dialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        closeOnBackdrop={false}
        title="New incident"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Cancel</Button>
            <Button type="submit" form="incident-form" disabled={submitting || !createForm.title}>{submitting ? 'Creating…' : 'Create incident'}</Button>
          </>
        }
      >
        <form id="incident-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleCreate(); }}>
          <div>
            <label htmlFor="inc_title" className={LABEL_CLASS}>Title</label>
            <input id="inc_title" required value={createForm.title} onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })} className={INPUT_CLASS} placeholder="What happened?" />
          </div>
          <div>
            <label htmlFor="inc_severity" className={LABEL_CLASS}>Severity</label>
            <select id="inc_severity" value={createForm.severity} onChange={(e) => setCreateForm({ ...createForm, severity: e.target.value })} className={INPUT_CLASS}>
              {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
          </div>
          <div>
            <label htmlFor="inc_description" className={LABEL_CLASS}>Description</label>
            <textarea id="inc_description" value={createForm.description} onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })} rows={3} className={TEXTAREA_CLASS} />
          </div>
          <div>
            <label htmlFor="inc_impact" className={LABEL_CLASS}>Impact</label>
            <textarea id="inc_impact" value={createForm.impact_description} onChange={(e) => setCreateForm({ ...createForm, impact_description: e.target.value })} rows={2} className={TEXTAREA_CLASS} placeholder="Who or what is affected?" />
          </div>
        </form>
      </Dialog>
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const TEXTAREA_CLASS = 'w-full bg-inset border border-line rounded-md px-3 py-2 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary resize-none';
const LABEL_CLASS = 'text-xs font-medium text-fg-muted block mb-1.5';
