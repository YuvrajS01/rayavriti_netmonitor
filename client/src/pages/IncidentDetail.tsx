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
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';

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
    return <div className="p-8 text-sm text-fg-muted">Loading incident…</div>;
  }

  if (!incident) {
    return (
      <div className="text-center py-20">
        <span className="material-symbols-outlined text-4xl text-fg-subtle" aria-hidden="true">error</span>
        <p className="text-fg-muted mt-3">Incident not found.</p>
        <Button variant="secondary" icon="arrow_back" onClick={() => navigate('/incidents')} className="mt-4">Back to incidents</Button>
      </div>
    );
  }

  const statusIdx = STATUS_FLOW.indexOf(incident.status);

  return (
    <div className="space-y-6">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div className="flex items-start gap-3">
          <IconButton icon="arrow_back" label="Back to incidents" onClick={() => navigate('/incidents')} className="mt-0.5" />
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-2xl font-semibold text-fg">{incident.title}</h1>
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

      <Card variant="low" className="p-5">
        <ol className="flex items-center" aria-label="Incident progress">
          {STATUS_FLOW.map((s, i) => {
            const done = i < statusIdx;
            const current = i === statusIdx;
            return (
              <li key={s} className="flex-1 flex items-center" aria-current={current ? 'step' : undefined}>
                <div className="flex flex-col items-center flex-1">
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold tabular-nums ${i <= statusIdx ? 'bg-primary text-on-primary' : 'bg-surface-3 text-fg-muted'} ${current ? 'ring-4 ring-primary/25' : ''}`}>
                    {done ? <span className="material-symbols-outlined text-sm" aria-hidden="true">check</span> : i + 1}
                  </div>
                  <span className={`text-xs mt-1 font-medium capitalize ${i <= statusIdx ? 'text-fg' : 'text-fg-muted'}`}>
                    {s}
                    {done && <span className="sr-only"> (done)</span>}
                  </span>
                </div>
                {i < STATUS_FLOW.length - 1 && <div className={`h-0.5 flex-1 mx-1 rounded ${i < statusIdx ? 'bg-primary' : 'bg-line'}`} aria-hidden="true" />}
              </li>
            );
          })}
        </ol>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 space-y-6">
          {[
            ['Description', incident.description],
            ['Impact', incident.impact_description],
            ['Resolution', incident.resolution],
            ['Root cause', incident.root_cause],
          ].map(([heading, body]) =>
            body ? (
              <Card key={heading} variant="low" className="p-5">
                <h2 className="font-semibold text-sm text-fg-muted mb-2">{heading}</h2>
                <p className="text-sm text-fg leading-relaxed">{body}</p>
              </Card>
            ) : null,
          )}

          <Card variant="low" className="overflow-hidden">
            <div className="p-4 border-b border-line">
              <h2 className="font-semibold text-sm text-fg-muted">Timeline</h2>
            </div>
            {timeline.length === 0 ? (
              <div className="p-8 text-sm text-fg-muted text-center">No timeline entries yet.</div>
            ) : (
              <ol className="divide-y divide-line">
                {timeline.map((entry) => (
                  <li key={entry.id} className="flex gap-3 p-4">
                    <span className="mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-surface-3">
                      <span className="material-symbols-outlined text-[16px] text-fg-muted" aria-hidden="true">{timelineIcon(entry.entryType)}</span>
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-fg">{entry.message}</p>
                      <div className="flex flex-wrap items-center gap-3 mt-1 text-xs text-fg-muted">
                        <span>{entry.author}</span>
                        <time dateTime={entry.createdAt}>{timeAgo(entry.createdAt)}</time>
                        {entry.oldValue && entry.newValue && (
                          <span>
                            <span className="line-through">{entry.oldValue}</span> → <span className="font-medium text-fg">{entry.newValue}</span>
                          </span>
                        )}
                      </div>
                    </div>
                  </li>
                ))}
              </ol>
            )}
            <form className="p-4 border-t border-line flex gap-3" onSubmit={(e) => { e.preventDefault(); handleAddNote(); }}>
              <label className="flex-1">
                <span className="sr-only">Add a note</span>
                <input
                  value={noteText}
                  onChange={(e) => setNoteText(e.target.value)}
                  placeholder="Add a note…"
                  className="w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary"
                />
              </label>
              <Button type="submit" icon="send" disabled={submitting || !noteText.trim()} className="flex-shrink-0">Add</Button>
            </form>
          </Card>
        </div>

        <div className="space-y-6">
          <Card variant="low" className="p-5">
            <h2 className="font-semibold text-sm text-fg-muted mb-4">Details</h2>
            <dl className="space-y-3">
              {[
                { label: 'Source', value: incident.source || 'manual' },
                { label: 'Duration', value: formatDuration(incident.duration_seconds) },
                { label: 'Devices affected', value: String(incident.affected_device_count) },
                { label: 'Started', value: formatTime(incident.started_at) },
                { label: 'Acknowledged', value: formatTime(incident.acknowledged_at) },
                { label: 'Resolved', value: formatTime(incident.resolved_at) },
                { label: 'Closed', value: formatTime(incident.closed_at) },
              ].map((row) => (
                <div key={row.label} className="flex justify-between items-center gap-3">
                  <dt className="text-xs text-fg-muted">{row.label}</dt>
                  <dd className="text-sm font-medium text-fg tabular-nums text-right">{row.value}</dd>
                </div>
              ))}
              <div className="flex justify-between items-center gap-3">
                <dt className="text-xs text-fg-muted">SLA</dt>
                <dd><StatusPill status={incident.sla_breached ? 'down' : 'up'}>{incident.sla_breached ? 'Breached' : 'Met'}</StatusPill></dd>
              </div>
            </dl>
          </Card>

          {devices.length > 0 && (
            <Card variant="low" className="overflow-hidden">
              <div className="p-4 border-b border-line">
                <h2 className="font-semibold text-sm text-fg-muted">Affected devices</h2>
              </div>
              <ul className="divide-y divide-line px-4">
                {devices.map((d) => (
                  <li key={d.id} className="py-3 flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-sm font-medium truncate">{d.name}</div>
                      <div className="text-xs text-fg-muted font-mono">{d.ip}</div>
                    </div>
                    <StatusPill status={d.status} />
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>
      </div>

      <Dialog
        open={showResolve}
        onClose={() => setShowResolve(false)}
        size="sm"
        closeOnBackdrop={false}
        title="Resolve incident"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowResolve(false)}>Cancel</Button>
            <Button type="submit" form="resolve-form" icon="check_circle" disabled={submitting || !resolveForm.resolution}>{submitting ? 'Resolving…' : 'Resolve'}</Button>
          </>
        }
      >
        <form id="resolve-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleAction('resolve', resolveForm); }}>
          <div>
            <label htmlFor="res_resolution" className={LABEL_CLASS}>Resolution</label>
            <textarea id="res_resolution" required value={resolveForm.resolution} onChange={(e) => setResolveForm({ ...resolveForm, resolution: e.target.value })} rows={3} className="w-full bg-inset border border-line rounded-md px-3 py-2 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary resize-none" placeholder="How was it resolved?" />
          </div>
          <div>
            <label htmlFor="res_cause" className={LABEL_CLASS}>Root cause</label>
            <input id="res_cause" value={resolveForm.rootCause} onChange={(e) => setResolveForm({ ...resolveForm, rootCause: e.target.value })} className={INPUT_CLASS} placeholder="What caused it?" />
          </div>
          <div>
            <label htmlFor="res_category" className={LABEL_CLASS}>Category</label>
            <select id="res_category" value={resolveForm.rootCauseCategory} onChange={(e) => setResolveForm({ ...resolveForm, rootCauseCategory: e.target.value })} className={INPUT_CLASS}>
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
        </form>
      </Dialog>
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const LABEL_CLASS = 'text-xs font-medium text-fg-muted block mb-1.5';
