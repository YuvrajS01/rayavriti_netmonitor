import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createIncident, getIncidentStats, getIncidents, runIncidentAction, type Incident, type IncidentAction, type IncidentStats } from '../api/incidents';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';
import { severityIcon, severityTextColor, severityBgColor, statusTextColor } from '../utils/colors';
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
    <div className="space-y-8">
      <SectionHeader
        title="Incident Manager"
        subtitle="Track, assign, and resolve network incidents with SLA tracking."
        action={<Button icon="add" onClick={() => setShowCreate(true)}>New Incident</Button>}
      />

      {stats && (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
          <StatCard label="Open" value={stats.open} icon="radio_button_checked" color="text-error" />
          <StatCard label="Acknowledged" value={stats.acknowledged} icon="visibility" color="text-warning" />
          <StatCard label="Resolved" value={stats.resolved} icon="check_circle" color="text-success" />
          <StatCard label="SLA breached" value={stats.sla_breached} icon="gpp_bad" color="text-error" />
          <StatCard label="Avg duration" value={formatDuration(stats.avg_duration_seconds)} icon="schedule" />
        </div>
      )}

      <GanttTimeline title="Recent incident windows" items={incidents.map((incident) => ({ id: incident.id, label: incident.title, start: incident.started_at, end: incident.resolved_at || incident.closed_at, color: incident.severity === 'critical' ? 'var(--color-error)' : incident.severity === 'major' ? 'var(--color-warning)' : 'var(--color-info)' }))} />

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-line flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">crisis_alert</span>
            </div>
            <div>
              <h2 className="font-sans font-semibold text-lg">Incidents</h2>
              <p className="text-xs text-fg-muted">{filtered.length} incidents</p>
            </div>
          </div>
          <div className="flex gap-3 flex-wrap">
            <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2.5 text-xs text-fg outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All severities</option>
              {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2.5 text-xs text-fg outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All statuses</option>
              {STATUSES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
            <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search incidents..." className="bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none md:w-56" />
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="crisis_alert" title="No incidents" description="All clear — no active incidents on the network." />
        ) : (
          <div className="divide-y divide-line">
            {filtered.map((inc) => (
              <article key={inc.id} className="p-5 hover:bg-surface-1/50 transition-colors cursor-pointer" onClick={() => navigate(`/incidents/${inc.id}`)}>
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${severityBgColor(inc.severity)}`}>
                      <span className={`material-symbols-outlined text-xl ${severityTextColor(inc.severity)}`}>{severityIcon(inc.severity)}</span>
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <h3 className="font-sans font-semibold text-lg truncate">{inc.title}</h3>
                        {inc.sla_breached && (
                          <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-error/10 text-error">SLA breach</span>
                        )}
                      </div>
                      <p className="text-xs text-fg-muted mt-0.5">
                        {inc.source || 'manual'} · #{inc.id} · Started {timeAgo(inc.started_at)}
                      </p>
                      {inc.description && <p className="text-sm text-fg-muted mt-2 line-clamp-2">{inc.description}</p>}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:max-w-xl flex-shrink-0">
                    <div>
                      <div className="text-xs text-fg-muted">Severity</div>
                      <div className={`text-sm font-semibold capitalize ${severityTextColor(inc.severity)}`}>{inc.severity}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">Status</div>
                      <div className={`text-sm font-semibold capitalize ${statusTextColor(inc.status)}`}>{inc.status}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">Devices</div>
                      <div className="text-sm font-medium">{inc.affected_device_count}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">Duration</div>
                      <div className="text-sm font-medium">{formatDuration(inc.duration_seconds)}</div>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 mt-3 pt-3 border-t border-line">
                  {inc.status === 'open' && (
                    <button onClick={(e) => { e.stopPropagation(); handleAcknowledge(inc.id); }} className="text-xs font-semibold text-warning hover:bg-warning/10 px-3 py-1 rounded transition-colors">Acknowledge</button>
                  )}
                  {(inc.status === 'open' || inc.status === 'acknowledged') && (
                    <button onClick={(e) => { e.stopPropagation(); handleResolve(inc.id); }} className="text-xs font-semibold text-success hover:bg-success/10 px-3 py-1 rounded transition-colors">Resolve</button>
                  )}
                  {inc.status === 'resolved' && (
                    <button onClick={(e) => { e.stopPropagation(); handleClose(inc.id); }} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Close</button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowCreate(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">New Incident</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-xs text-fg-muted block mb-1">Title</label>
                <input value={createForm.title} onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="What happened?" />
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Severity</label>
                <select value={createForm.severity} onChange={(e) => setCreateForm({ ...createForm, severity: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                  {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
                </select>
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Description</label>
                <textarea value={createForm.description} onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })} rows={3} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none resize-none" />
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Impact</label>
                <textarea value={createForm.impact_description} onChange={(e) => setCreateForm({ ...createForm, impact_description: e.target.value })} rows={2} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none resize-none" placeholder="Who/what is affected?" />
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowCreate(false)} className="flex-1 py-3 text-xs font-sans font-semibold text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleCreate} disabled={submitting || !createForm.title} className="flex-1 py-3 text-xs font-sans font-semibold text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Creating...' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
