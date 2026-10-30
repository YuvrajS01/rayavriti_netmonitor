import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  addIncidentNote, getIncident, getIncidentDevices, getIncidentTimeline, runIncidentAction,
  type Incident, type IncidentAction, type IncidentDevice, type IncidentTimelineEntry,
} from '../api/incidents';
import { queryKeys } from '../api/queryKeys';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import { useToast } from '../components/ui/useToast';
import { lifecycleMeta, severityMeta } from '../utils/status';
import { StatusPill } from '../components/ui/Status';

function formatDuration(sec: number | null): string {
  if (sec == null) return '-';
  if (sec < 60) return `${Math.round(sec)}s`;
  if (sec < 3600) return `${Math.round(sec / 60)}m`;
  const hrs = Math.floor(sec / 3600);
  const mins = Math.round((sec % 3600) / 60);
  return mins > 0 ? `${hrs}h ${mins}m` : `${hrs}h`;
}

function formatTime(ts: string | null): string {
  if (!ts) return '-';
  return new Date(ts).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short' });
}

function timeAgo(ts: string | null): string {
  if (!ts) return 'Never';
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function timelineIcon(type: string): string {
  if (type === 'created') return 'add_circle';
  if (type === 'status_change') return 'swap_horiz';
  if (type === 'note') return 'sticky_note_2';
  if (type === 'assignment') return 'person_add';
  if (type === 'device_linked') return 'devices';
  return 'timeline';
}

const STATUS_FLOW = ['open', 'acknowledged', 'resolved', 'closed'];
const EMPTY_TIMELINE: IncidentTimelineEntry[] = [];
const EMPTY_DEVICES: IncidentDevice[] = [];

export default function IncidentDetail() {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [noteText, setNoteText] = useState('');
  const [resolveForm, setResolveForm] = useState({ resolution: '', rootCause: '', rootCauseCategory: '' });
  const [showResolve, setShowResolve] = useState(false);

  const incidentQuery = useQuery({
    queryKey: queryKeys.incidents.detail(id),
    queryFn: ({ signal }) => getIncident(id, { signal }),
    enabled: Boolean(id),
  });
  // Timeline and devices are secondary: a failure renders them empty.
  const timelineQuery = useQuery({
    queryKey: queryKeys.incidents.timeline(id),
    queryFn: ({ signal }) => getIncidentTimeline(id, { signal }),
    enabled: Boolean(id),
  });
  const devicesQuery = useQuery({
    queryKey: queryKeys.incidents.devices(id),
    queryFn: ({ signal }) => getIncidentDevices(id, { signal }),
    enabled: Boolean(id),
  });

  const loading = Boolean(id) && incidentQuery.isPending;
  const incident: Incident | null = incidentQuery.data?.data ?? null;
  const timeline = timelineQuery.data?.data || EMPTY_TIMELINE;
  const devices = devicesQuery.data?.data || EMPTY_DEVICES;

  const refreshIncident = () => queryClient.invalidateQueries({ queryKey: queryKeys.incidents.all });

  const actionMutation = useMutation({
    mutationFn: ({ action, body }: { action: IncidentAction; body?: Record<string, unknown> }) => runIncidentAction(id, action, body),
    onSuccess: (_res, { action }) => {
      addToast(`Incident ${action}`, 'success');
      if (action === 'resolve') setShowResolve(false);
      return refreshIncident();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Action failed', 'error'),
  });

  const noteMutation = useMutation({
    mutationFn: (message: string) => addIncidentNote(id, message),
    onSuccess: () => {
      setNoteText('');
      addToast('Note added', 'success');
      return refreshIncident();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Failed', 'error'),
  });

  const submitting = actionMutation.isPending || noteMutation.isPending;

  const handleAction = (action: IncidentAction, body?: Record<string, unknown>) => {
    actionMutation.mutate({ action, body });
  };

  const handleAddNote = () => {
    if (!noteText.trim()) return;
    noteMutation.mutate(noteText.trim());
  };

  if (loading) {
    return <div className="p-8 text-sm text-fg-muted">Loading incident...</div>;
  }

  if (!incident) {
    return (
      <div className="text-center py-20">
        <span className="material-symbols-outlined text-4xl text-fg-subtle">error</span>
        <p className="text-fg-muted mt-3">Incident not found.</p>
        <Button variant="ghost" icon="arrow_back" onClick={() => navigate('/incidents')} className="mt-4">Back to Incidents</Button>
      </div>
    );
  }

  const statusIdx = STATUS_FLOW.indexOf(incident.status);

  return (
    <div className="space-y-8">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div className="flex items-start gap-4">
          <button onClick={() => navigate('/incidents')} className="mt-1 text-fg-muted hover:text-fg transition-colors">
            <span className="material-symbols-outlined">arrow_back</span>
          </button>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="font-sans text-2xl font-semibold text-fg">{incident.title}</h1>
              <StatusPill status={severityMeta(incident.severity)} />
              <StatusPill status={lifecycleMeta(incident.status)} />
              {incident.sla_breached && <StatusPill status="down">SLA breach</StatusPill>}
            </div>
            <p className="text-fg-muted text-sm mt-1">#{incident.id} · Started {formatTime(incident.started_at)}</p>
          </div>
        </div>
        <div className="flex gap-2 flex-wrap">
          {incident.status === 'open' && (
            <Button icon="visibility" variant="secondary" onClick={() => handleAction('acknowledge')} disabled={submitting}>Acknowledge</Button>
          )}
          {(incident.status === 'open' || incident.status === 'acknowledged') && (
            <Button icon="check_circle" onClick={() => setShowResolve(true)} disabled={submitting}>Resolve</Button>
          )}
          {incident.status === 'resolved' && (
            <Button icon="archive" onClick={() => handleAction('close')} disabled={submitting}>Close</Button>
          )}
        </div>
      </header>

      {/* Status Progress Bar */}
      <Card variant="low" className="p-5">
        <div className="flex items-center gap-0">
          {STATUS_FLOW.map((s, i) => (
            <div key={s} className="flex-1 flex items-center">
              <div className="flex flex-col items-center flex-1">
                 <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold transition-colors ${i <= statusIdx ? 'bg-primary text-on-primary' : 'bg-inset text-fg-subtle'} ${i === statusIdx ? 'ring-4 ring-primary/25' : ''}`}>
                   {i < statusIdx ? <span className="material-symbols-outlined text-sm">check</span> : i + 1}
                 </div>
                <span className={`text-xs mt-1 font-semibold ${i <= statusIdx ? 'text-primary': 'text-fg-subtle'}`}>{s}</span>
              </div>
              {i < STATUS_FLOW.length - 1 && <div className={`h-0.5 flex-1 mx-1 rounded ${i < statusIdx ? 'bg-primary' : 'bg-inset'}`} />}
            </div>
          ))}
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Details */}
        <div className="lg:col-span-2 space-y-6">
          {incident.description && (
            <Card variant="low" className="p-5">
              <h3 className="font-sans font-semibold text-sm text-fg-muted mb-2">Description</h3>
              <p className="text-sm text-fg leading-relaxed">{incident.description}</p>
            </Card>
          )}

          {incident.impact_description && (
            <Card variant="low" className="p-5">
              <h3 className="font-sans font-semibold text-sm text-fg-muted mb-2">Impact</h3>
              <p className="text-sm text-fg leading-relaxed">{incident.impact_description}</p>
            </Card>
          )}

          {incident.resolution && (
            <Card variant="low" className="p-5">
              <h3 className="font-sans font-semibold text-sm text-fg-muted mb-2">Resolution</h3>
              <p className="text-sm text-fg leading-relaxed">{incident.resolution}</p>
            </Card>
          )}

          {incident.root_cause && (
            <Card variant="low" className="p-5">
              <h3 className="font-sans font-semibold text-sm text-fg-muted mb-2">Root cause</h3>
              <p className="text-sm text-fg leading-relaxed">{incident.root_cause}</p>
            </Card>
          )}

          {/* Timeline */}
          <Card variant="low" className="overflow-hidden">
            <div className="p-5 border-b border-line">
              <h3 className="font-sans font-semibold text-sm text-fg-muted">Timeline</h3>
            </div>
            {timeline.length === 0 ? (
              <div className="p-8 text-sm text-fg-muted text-center">No timeline entries yet.</div>
            ) : (
              <div className="relative">
                <div className="absolute left-[29px] top-0 bottom-0 w-px bg-line/30" />
                <div className="divide-y divide-line">
                  {timeline.map((entry) => (
                    <div key={entry.id} className="flex gap-4 p-5 relative">
                      <div className="w-[14px] h-[14px] rounded-full bg-surface-1 border-2 border-line flex-shrink-0 mt-1 z-10" style={{ marginLeft: '22px' }} />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className={`material-symbols-outlined text-sm ${entry.entryType === 'note' ? 'text-info' : entry.entryType === 'status_change' ? 'text-warning' : 'text-primary'}`}>{timelineIcon(entry.entryType)}</span>
                          <span className="text-sm font-medium text-fg">{entry.message}</span>
                        </div>
                        <div className="flex items-center gap-3 mt-1">
                          <span className="text-xs text-fg-muted">{entry.author}</span>
                          <span className="text-xs text-fg-muted">{timeAgo(entry.createdAt)}</span>
                          {entry.oldValue && entry.newValue && (
                            <span className="text-xs text-fg-muted">
                              <span className="line-through opacity-50">{entry.oldValue}</span> → <span className="font-medium">{entry.newValue}</span>
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {/* Add Note */}
            <div className="p-4 border-t border-line flex gap-3">
              <input
                value={noteText}
                onChange={(e) => setNoteText(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleAddNote(); } }}
                placeholder="Add a note..."
                className="flex-1 bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none"
              />
              <Button icon="send" onClick={handleAddNote} disabled={submitting || !noteText.trim()} className="flex-shrink-0">Add</Button>
            </div>
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          <Card variant="low" className="p-5 space-y-4">
            <h3 className="font-sans font-semibold text-sm text-fg-muted">Details</h3>
            {[
              { label: 'Source', value: incident.source || 'manual' },
              { label: 'Duration', value: formatDuration(incident.duration_seconds) },
              { label: 'Devices Affected', value: String(incident.affected_device_count) },
              { label: 'SLA', value: incident.sla_breached ? 'Breached' : 'Met', color: incident.sla_breached ? 'text-error' : 'text-success' },
              { label: 'Started', value: formatTime(incident.started_at) },
              { label: 'Acknowledged', value: formatTime(incident.acknowledged_at) },
              { label: 'Resolved', value: formatTime(incident.resolved_at) },
              { label: 'Closed', value: formatTime(incident.closed_at) },
            ].map((row) => (
              <div key={row.label} className="flex justify-between items-center">
                <span className="text-xs text-fg-muted">{row.label}</span>
                <span className={`text-sm font-medium ${row.color || 'text-fg'}`}>{row.value}</span>
              </div>
            ))}
          </Card>

          {devices.length > 0 && (
            <Card variant="low" className="overflow-hidden">
              <div className="p-5 border-b border-line">
                <h3 className="font-sans font-semibold text-sm text-fg-muted">Affected devices</h3>
              </div>
              <div className="p-5">
                <div className="grid grid-cols-4 sm:grid-cols-6 gap-2 mb-4" aria-label="Impacted topology">
                  {devices.map((d, i) => {
                    const color = d.status === 'down' ? 'var(--color-error)' : d.status === 'warning' || d.status === 'degraded' ? 'var(--color-warning)' : 'var(--color-success)';
                    return (
                      <div key={d.id} className="relative aspect-square rounded-md border border-line bg-inset flex items-center justify-center" title={`${d.name} · ${d.status}`} style={{ animationDelay: `${i * 40}ms` }}>
                        <span className="w-2.5 h-2.5 rounded-full" style={{ background: color, boxShadow: `0 0 6px ${color}` }} />
                        {i < devices.length - 1 && <span className="absolute -right-2 top-1/2 w-2 h-px bg-line/40" />}
                      </div>
                    );
                  })}
                </div>
                <div className="divide-y divide-line">
                  {devices.map((d) => (
                    <div key={d.id} className="px-0 py-3 flex items-center justify-between hover:bg-surface-1/50 transition-colors">
                      <div>
                        <div className="text-sm font-medium">{d.name}</div>
                        <div className="text-xs text-fg-muted font-mono">{d.ip}</div>
                      </div>
                      <StatusPill status={d.status} />
                    </div>
                  ))}
                </div>
              </div>
            </Card>
          )}
        </div>
      </div>

      {showResolve && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowResolve(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-md max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">Resolve Incident</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-xs text-fg-muted block mb-1">Resolution</label>
                <textarea value={resolveForm.resolution} onChange={(e) => setResolveForm({ ...resolveForm, resolution: e.target.value })} rows={3} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none resize-none" placeholder="How was it resolved?" />
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Root cause</label>
                <input value={resolveForm.rootCause} onChange={(e) => setResolveForm({ ...resolveForm, rootCause: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="What caused it?" />
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Category</label>
                <select value={resolveForm.rootCauseCategory} onChange={(e) => setResolveForm({ ...resolveForm, rootCauseCategory: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                  <option value="">Select category</option>
                  <option value="hardware">Hardware</option>
                  <option value="software">Software</option>
                  <option value="network">Network</option>
                  <option value="configuration">Configuration</option>
                  <option value="power">Power</option>
                  <option value="external">External</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowResolve(false)} className="flex-1 py-3 text-xs font-sans font-semibold text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={() => handleAction('resolve', resolveForm)} disabled={submitting || !resolveForm.resolution} className="flex-1 py-3 text-xs font-sans font-semibold text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Resolving...' : 'Resolve'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
