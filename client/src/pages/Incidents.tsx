import { useState, useEffect, useMemo, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { v1, wrap } from '../api/http';
import { useSocket } from '../hooks/useSocket';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';
import { severityIcon, severityTextColor, severityBgColor, statusTextColor } from '../utils/colors';

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
}

interface IncidentStats {
  open: number;
  acknowledged: number;
  resolved: number;
  closed: number;
  sla_breached: number;
  total: number;
  avg_duration_seconds: number | null;
  avg_response_seconds: number | null;
}

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
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [stats, setStats] = useState<IncidentStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [severityFilter, setSeverityFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ title: '', description: '', severity: 'minor', impact_description: '' });
  const [submitting, setSubmitting] = useState(false);

  const load = async () => {
    setLoading(true);
    const [listRes, statsRes] = await Promise.all([
      v1.get('/incidents'),
      v1.get('/incidents/stats').catch(() => ({ data: null })),
    ]);
    setIncidents(wrap<Incident[]>(listRes.data).data || []);
    if (statsRes.data) setStats(wrap<IncidentStats>(statsRes.data).data);
    setLoading(false);
  };

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const [listRes, statsRes] = await Promise.all([
          v1.get('/incidents'),
          v1.get('/incidents/stats').catch(() => ({ data: null })),
        ]);
        if (active) setIncidents(wrap<Incident[]>(listRes.data).data || []);
        if (active && statsRes.data) setStats(wrap<IncidentStats>(statsRes.data).data);
      } catch {
        if (active) {
          setIncidents([]);
          setStats(null);
        }
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, []);

  const lastRefresh = useRef(0);
  useSocket({
    onAlertTriggered: () => {
      const now = Date.now();
      if (now - lastRefresh.current > 10_000) { lastRefresh.current = now; load(); }
    },
    onAlertResolved: () => {
      const now = Date.now();
      if (now - lastRefresh.current > 10_000) { lastRefresh.current = now; load(); }
    },
  });

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return incidents.filter((i) => {
      const matchSearch = !needle || i.title.toLowerCase().includes(needle) || i.description?.toLowerCase().includes(needle);
      const matchSev = severityFilter === 'all' || i.severity === severityFilter;
      const matchStatus = statusFilter === 'all' || i.status === statusFilter;
      return matchSearch && matchSev && matchStatus;
    });
  }, [incidents, search, severityFilter, statusFilter]);

  const handleCreate = async () => {
    setSubmitting(true);
    try {
      await v1.post('/incidents', createForm);
      addToast('Incident created', 'success');
      setShowCreate(false);
      setCreateForm({ title: '', description: '', severity: 'minor', impact_description: '' });
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Create failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleAcknowledge = async (id: number) => {
    try {
      await v1.post(`/incidents/${id}/acknowledge`);
      addToast('Incident acknowledged', 'success');
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed', 'error');
    }
  };

  const handleResolve = async (id: number) => {
    try {
      await v1.post(`/incidents/${id}/resolve`, { resolution: 'Resolved via dashboard' });
      addToast('Incident resolved', 'success');
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed', 'error');
    }
  };

  const handleClose = async (id: number) => {
    try {
      await v1.post(`/incidents/${id}/close`);
      addToast('Incident closed', 'success');
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed', 'error');
    }
  };

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
          <StatCard label="SLA Breached" value={stats.sla_breached} icon="gpp_bad" color="text-error" />
          <StatCard label="Avg Duration" value={formatDuration(stats.avg_duration_seconds)} icon="schedule" />
        </div>
      )}

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">crisis_alert</span>
            </div>
            <div>
              <h2 className="font-headline font-bold text-lg">Incidents</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{filtered.length} incidents</p>
            </div>
          </div>
          <div className="flex gap-3 flex-wrap">
            <select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All severities</option>
              {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All statuses</option>
              {STATUSES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
            </select>
            <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search incidents..." className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-56" />
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="crisis_alert" title="No incidents" description="All clear — no active incidents on the network." />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filtered.map((inc) => (
              <article key={inc.id} className="p-5 hover:bg-surface-container-high/50 transition-colors cursor-pointer" onClick={() => navigate(`/incidents/${inc.id}`)}>
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${severityBgColor(inc.severity)}`}>
                      <span className={`material-symbols-outlined text-xl ${severityTextColor(inc.severity)}`}>{severityIcon(inc.severity)}</span>
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <h3 className="font-headline font-bold text-lg truncate">{inc.title}</h3>
                        {inc.sla_breached && (
                          <span className="text-[10px] font-bold uppercase tracking-wide px-2 py-0.5 rounded-full bg-error/10 text-error">SLA Breach</span>
                        )}
                      </div>
                      <p className="text-xs text-on-surface-variant mt-0.5">
                        {inc.source || 'manual'} · #{inc.id} · Started {timeAgo(inc.started_at)}
                      </p>
                      {inc.description && <p className="text-sm text-on-surface-variant mt-2 line-clamp-2">{inc.description}</p>}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:max-w-xl flex-shrink-0">
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Severity</div>
                      <div className={`text-sm font-bold capitalize ${severityTextColor(inc.severity)}`}>{inc.severity}</div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Status</div>
                      <div className={`text-sm font-bold capitalize ${statusTextColor(inc.status)}`}>{inc.status}</div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Devices</div>
                      <div className="text-sm font-medium">{inc.affected_device_count}</div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Duration</div>
                      <div className="text-sm font-medium">{formatDuration(inc.duration_seconds)}</div>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 mt-3 pt-3 border-t border-outline-variant/10">
                  {inc.status === 'open' && (
                    <button onClick={(e) => { e.stopPropagation(); handleAcknowledge(inc.id); }} className="text-xs font-bold text-warning hover:bg-warning/10 px-3 py-1 rounded transition-colors">Acknowledge</button>
                  )}
                  {(inc.status === 'open' || inc.status === 'acknowledged') && (
                    <button onClick={(e) => { e.stopPropagation(); handleResolve(inc.id); }} className="text-xs font-bold text-success hover:bg-success/10 px-3 py-1 rounded transition-colors">Resolve</button>
                  )}
                  {inc.status === 'resolved' && (
                    <button onClick={(e) => { e.stopPropagation(); handleClose(inc.id); }} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Close</button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowCreate(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0"><h2 className="font-headline text-lg font-bold">New Incident</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Title</label>
                <input value={createForm.title} onChange={(e) => setCreateForm({ ...createForm, title: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="What happened?" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Severity</label>
                <select value={createForm.severity} onChange={(e) => setCreateForm({ ...createForm, severity: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                  {SEVERITIES.map((s) => <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>)}
                </select>
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Description</label>
                <textarea value={createForm.description} onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })} rows={3} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none resize-none" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Impact</label>
                <textarea value={createForm.impact_description} onChange={(e) => setCreateForm({ ...createForm, impact_description: e.target.value })} rows={2} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none resize-none" placeholder="Who/what is affected?" />
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowCreate(false)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleCreate} disabled={submitting || !createForm.title} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Creating...' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
