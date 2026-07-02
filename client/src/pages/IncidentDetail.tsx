import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { v1, wrap } from '../api/http';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import { useToast } from '../components/ui/useToast';
import { severityTextColor, severityBgColor, statusTextColor, statusBgColor } from '../utils/colors';

interface Incident {
  id: number;
  title: string;
  description: string;
  severity: string;
  status: string;
  source: string;
  assigned_to: number | null;
  location_id: number | null;
  impact_description: string;
  affected_device_count: number;
  started_at: string;
  acknowledged_at: string | null;
  resolved_at: string | null;
  closed_at: string | null;
  duration_seconds: number | null;
  sla_breached: boolean;
  root_cause: string | null;
  resolution: string | null;
  created_by: number | null;
}

interface TimelineEntry {
  id: number;
  incidentId: number;
  entryType: string;
  oldValue: string | null;
  newValue: string | null;
  message: string;
  author: string;
  createdAt: string;
}

interface IncidentDevice {
  id: number;
  name: string;
  ip: string;
  status: string;
}

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

export default function IncidentDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const [incident, setIncident] = useState<Incident | null>(null);
  const [timeline, setTimeline] = useState<TimelineEntry[]>([]);
  const [devices, setDevices] = useState<IncidentDevice[]>([]);
  const [loading, setLoading] = useState(true);
  const [noteText, setNoteText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [resolveForm, setResolveForm] = useState({ resolution: '', rootCause: '', rootCauseCategory: '' });
  const [showResolve, setShowResolve] = useState(false);

  useEffect(() => {
    let active = true;
    (async () => {
      if (!id) return;
      setLoading(true);
      try {
        const [incRes, tlRes, devRes] = await Promise.all([
          v1.get(`/incidents/${id}`),
          v1.get(`/incidents/${id}/timeline`).catch(() => ({ data: { data: [] } })),
          v1.get(`/incidents/${id}/devices`).catch(() => ({ data: { data: [] } })),
        ]);
        if (active) setIncident(wrap<Incident>(incRes.data).data);
        if (active) setTimeline(wrap<TimelineEntry[]>(tlRes.data).data || []);
        if (active) setDevices(wrap<IncidentDevice[]>(devRes.data).data || []);
      } catch {
        if (active) {
          setIncident(null);
          setTimeline([]);
          setDevices([]);
        }
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, [id]);

  const load = async () => {
    if (!id) return;
    const [incRes, tlRes, devRes] = await Promise.all([
      v1.get(`/incidents/${id}`),
      v1.get(`/incidents/${id}/timeline`).catch(() => ({ data: { data: [] } })),
      v1.get(`/incidents/${id}/devices`).catch(() => ({ data: { data: [] } })),
    ]);
    setIncident(wrap<Incident>(incRes.data).data);
    setTimeline(wrap<TimelineEntry[]>(tlRes.data).data || []);
    setDevices(wrap<IncidentDevice[]>(devRes.data).data || []);
  };

  const handleAction = async (action: string, body?: Record<string, unknown>) => {
    setSubmitting(true);
    try {
      await v1.post(`/incidents/${id}/${action}`, body || {});
      addToast(`Incident ${action}`, 'success');
      if (action === 'resolve') setShowResolve(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Action failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleAddNote = async () => {
    if (!noteText.trim()) return;
    setSubmitting(true);
    try {
      await v1.post(`/incidents/${id}/note`, { message: noteText.trim() });
      setNoteText('');
      addToast('Note added', 'success');
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div className="p-8 text-sm text-on-surface-variant">Loading incident...</div>;
  }

  if (!incident) {
    return (
      <div className="text-center py-20">
        <span className="material-symbols-outlined text-4xl text-outline">error</span>
        <p className="text-on-surface-variant mt-3">Incident not found.</p>
        <Button variant="ghost" icon="arrow_back" onClick={() => navigate('/incidents')} className="mt-4">Back to Incidents</Button>
      </div>
    );
  }

  const statusIdx = STATUS_FLOW.indexOf(incident.status);

  return (
    <div className="space-y-8">
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div className="flex items-start gap-4">
          <button onClick={() => navigate('/incidents')} className="mt-1 text-on-surface-variant hover:text-on-surface transition-colors">
            <span className="material-symbols-outlined">arrow_back</span>
          </button>
          <div>
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="font-headline text-2xl font-semibold text-on-surface">{incident.title}</h1>
              <span className={`text-xs font-semibold uppercase px-2.5 py-0.5 rounded-full ${severityBgColor(incident.severity)} ${severityTextColor(incident.severity)}`}>{incident.severity}</span>
              <span className={`text-xs font-semibold uppercase px-2.5 py-0.5 rounded-full ${statusBgColor(incident.status)} ${statusTextColor(incident.status)}`}>{incident.status}</span>
              {incident.sla_breached && <span className="text-xs font-semibold uppercase px-2.5 py-0.5 rounded-full bg-error/10 text-error">SLA Breach</span>}
            </div>
            <p className="text-on-surface-variant text-sm mt-1">#{incident.id} · Started {formatTime(incident.started_at)}</p>
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
                <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold ${i <= statusIdx ? 'bg-primary text-on-primary' : 'bg-surface-container-lowest text-outline'}`}>
                  {i < statusIdx ? <span className="material-symbols-outlined text-sm">check</span> : i + 1}
                </div>
                <span className={`text-[10px] mt-1 uppercase tracking-wide font-semibold ${i <= statusIdx ? 'text-primary' : 'text-outline'}`}>{s}</span>
              </div>
              {i < STATUS_FLOW.length - 1 && <div className={`h-0.5 flex-1 mx-1 rounded ${i < statusIdx ? 'bg-primary' : 'bg-surface-container-lowest'}`} />}
            </div>
          ))}
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Details */}
        <div className="lg:col-span-2 space-y-6">
          {incident.description && (
            <Card variant="low" className="p-5">
              <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant mb-2">Description</h3>
              <p className="text-sm text-on-surface leading-relaxed">{incident.description}</p>
            </Card>
          )}

          {incident.impact_description && (
            <Card variant="low" className="p-5">
              <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant mb-2">Impact</h3>
              <p className="text-sm text-on-surface leading-relaxed">{incident.impact_description}</p>
            </Card>
          )}

          {incident.resolution && (
            <Card variant="low" className="p-5">
              <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant mb-2">Resolution</h3>
              <p className="text-sm text-on-surface leading-relaxed">{incident.resolution}</p>
            </Card>
          )}

          {incident.root_cause && (
            <Card variant="low" className="p-5">
              <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant mb-2">Root Cause</h3>
              <p className="text-sm text-on-surface leading-relaxed">{incident.root_cause}</p>
            </Card>
          )}

          {/* Timeline */}
          <Card variant="low" className="overflow-hidden">
            <div className="p-5 border-b border-outline-variant/20">
              <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant">Timeline</h3>
            </div>
            {timeline.length === 0 ? (
              <div className="p-8 text-sm text-on-surface-variant text-center">No timeline entries yet.</div>
            ) : (
              <div className="relative">
                <div className="absolute left-[29px] top-0 bottom-0 w-px bg-outline-variant/30" />
                <div className="divide-y divide-outline-variant/10">
                  {timeline.map((entry) => (
                    <div key={entry.id} className="flex gap-4 p-5 relative">
                      <div className="w-[14px] h-[14px] rounded-full bg-surface-container-low border-2 border-outline-variant/50 flex-shrink-0 mt-1 z-10" style={{ marginLeft: '22px' }} />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className={`material-symbols-outlined text-sm ${entry.entryType === 'note' ? 'text-info' : entry.entryType === 'status_change' ? 'text-warning' : 'text-primary'}`}>{timelineIcon(entry.entryType)}</span>
                          <span className="text-sm font-medium text-on-surface">{entry.message}</span>
                        </div>
                        <div className="flex items-center gap-3 mt-1">
                          <span className="text-xs text-on-surface-variant">{entry.author}</span>
                          <span className="text-xs text-on-surface-variant">{timeAgo(entry.createdAt)}</span>
                          {entry.oldValue && entry.newValue && (
                            <span className="text-xs text-on-surface-variant">
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
            <div className="p-4 border-t border-outline-variant/20 flex gap-3">
              <input
                value={noteText}
                onChange={(e) => setNoteText(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleAddNote(); } }}
                placeholder="Add a note..."
                className="flex-1 bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
              />
              <Button icon="send" onClick={handleAddNote} disabled={submitting || !noteText.trim()} className="flex-shrink-0">Add</Button>
            </div>
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          <Card variant="low" className="p-5 space-y-4">
            <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant">Details</h3>
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
                <span className="text-xs text-on-surface-variant uppercase tracking-wide">{row.label}</span>
                <span className={`text-sm font-medium ${row.color || 'text-on-surface'}`}>{row.value}</span>
              </div>
            ))}
          </Card>

          {devices.length > 0 && (
            <Card variant="low" className="overflow-hidden">
              <div className="p-5 border-b border-outline-variant/20">
                <h3 className="font-headline font-semibold text-sm uppercase tracking-wide text-on-surface-variant">Affected Devices</h3>
              </div>
              <div className="divide-y divide-outline-variant/10">
                {devices.map((d) => (
                  <div key={d.id} className="px-5 py-3 flex items-center justify-between hover:bg-surface-container-low/50 transition-colors">
                    <div>
                      <div className="text-sm font-medium">{d.name}</div>
                      <div className="text-xs text-on-surface-variant font-data">{d.ip}</div>
                    </div>
                    <span className={`text-[10px] font-semibold uppercase px-2 py-0.5 rounded-full ${statusBgColor(d.status)}/10 ${statusTextColor(d.status)}`}>{d.status}</span>
                  </div>
                ))}
              </div>
            </Card>
          )}
        </div>
      </div>

      {showResolve && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowResolve(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-md max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0"><h2 className="font-headline text-lg font-semibold">Resolve Incident</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Resolution</label>
                <textarea value={resolveForm.resolution} onChange={(e) => setResolveForm({ ...resolveForm, resolution: e.target.value })} rows={3} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none resize-none" placeholder="How was it resolved?" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Root Cause</label>
                <input value={resolveForm.rootCause} onChange={(e) => setResolveForm({ ...resolveForm, rootCause: e.target.value })} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="What caused it?" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Category</label>
                <select value={resolveForm.rootCauseCategory} onChange={(e) => setResolveForm({ ...resolveForm, rootCauseCategory: e.target.value })} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
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
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowResolve(false)} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-low transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={() => handleAction('resolve', resolveForm)} disabled={submitting || !resolveForm.resolution} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Resolving...' : 'Resolve'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
