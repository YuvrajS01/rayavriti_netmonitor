import { useState, useEffect, useCallback, useMemo } from 'react';
import { listPhase2, createPhase2, updatePhase2, type Phase2Row } from '../api/phase2';
import { v1 } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface MaintenanceWindow extends Phase2Row {
  id: number;
  name: string;
  description: string;
  scope_type: string;
  scope_value: string;
  schedule_type: string;
  start_time: string;
  end_time: string;
  recurrence_rule: string;
  suppress_alerts: boolean;
  suppress_notifications: boolean;
  show_maintenance_status: boolean;
  enabled: boolean;
}

function formatDate(ts: string | null): string {
  if (!ts) return '-';
  return new Date(ts).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short' });
}

function nowInRange(start: string | null, end: string | null): boolean {
  if (!start || !end) return false;
  const now = Date.now();
  return now >= new Date(start).getTime() && now <= new Date(end).getTime();
}

export default function Maintenance() {
  const { addToast } = useToast();
  const [windows, setWindows] = useState<MaintenanceWindow[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<MaintenanceWindow | null>(null);
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<MaintenanceWindow | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    const res = await listPhase2('/maintenance');
    setWindows((res.data || []) as MaintenanceWindow[]);
    setLoading(false);
  }, []);

  useEffect(() => { load().catch(() => setLoading(false)); }, [load]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return windows.filter((w) => {
      const matchSearch = !needle || (w.name as string)?.toLowerCase().includes(needle) || (w.scope_value as string)?.toLowerCase().includes(needle);
      const matchType = typeFilter === 'all' || w.schedule_type === typeFilter;
      return matchSearch && matchType;
    });
  }, [windows, search, typeFilter]);

  const stats = useMemo(() => ({
    total: windows.length,
    active: windows.filter((w) => w.enabled && nowInRange(w.start_time, w.end_time)).length,
    recurring: windows.filter((w) => w.schedule_type === 'recurring').length,
    suppress: windows.filter((w) => w.suppress_alerts).length,
  }), [windows]);

  const openCreate = () => {
    setEditing(null);
    setForm({ name: '', description: '', scope_type: 'global', scope_value: '*', schedule_type: 'one_time', start_time: '', end_time: '', suppress_alerts: true, suppress_notifications: true, show_maintenance_status: true, enabled: true });
    setShowForm(true);
  };
  const openEdit = (w: MaintenanceWindow) => { setEditing(w); setForm({ ...w }); setShowForm(true); };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      if (editing) {
        await updatePhase2('/maintenance', editing.id, form);
        addToast('Window updated', 'success');
      } else {
        await createPhase2('/maintenance', form);
        addToast('Window created', 'success');
      }
      setShowForm(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Save failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await v1.delete(`/maintenance/${deleteTarget.id}`);
      addToast('Window deleted', 'success');
      setDeleteTarget(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="Maintenance Calendar"
        subtitle="Plan downtime windows that suppress alerts and notifications."
        action={<Button icon="add" onClick={openCreate}>New Window</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Total" value={stats.total} icon="event_repeat" />
        <StatCard label="Currently Active" value={stats.active} icon="pending" color="text-warning" />
        <StatCard label="Recurring" value={stats.recurring} icon="repeat" />
        <StatCard label="Suppress Alerts" value={stats.suppress} icon="notifications_off" />
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">event_repeat</span>
            </div>
            <div>
              <h2 className="font-headline font-bold text-lg">Maintenance Windows</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{filtered.length} windows</p>
            </div>
          </div>
          <div className="flex gap-3">
            <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All types</option>
              <option value="one_time">One-time</option>
              <option value="recurring">Recurring</option>
            </select>
            <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-56" />
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="event_repeat" title="No maintenance windows" description="Schedule downtime to suppress false alerts." action={<Button icon="add" onClick={openCreate}>Create Window</Button>} />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filtered.map((w) => {
              const isActive = nowInRange(w.start_time, w.end_time);
              return (
                <article key={w.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${isActive ? 'bg-warning/10' : 'bg-surface-container-highest'}`}>
                        <span className={`material-symbols-outlined text-xl ${isActive ? 'text-warning' : 'text-on-surface-variant'}`}>{w.schedule_type === 'recurring' ? 'repeat' : 'event'}</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="font-headline font-bold text-lg truncate">{w.name}</h3>
                          {isActive && <span className="text-[10px] font-bold uppercase tracking-wide px-2 py-0.5 rounded-full bg-warning/10 text-warning">Active Now</span>}
                          {!w.enabled && <span className="text-[10px] font-bold uppercase tracking-wide px-2 py-0.5 rounded-full bg-outline/10 text-outline">Disabled</span>}
                        </div>
                        <p className="text-xs text-on-surface-variant mt-0.5 capitalize">{w.schedule_type?.replace('_', ' ')} · {w.scope_type}: {w.scope_value}</p>
                        {w.description && <p className="text-sm text-on-surface-variant mt-1 line-clamp-1">{w.description}</p>}
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 lg:max-w-lg flex-shrink-0">
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Start</div>
                        <div className="text-sm font-medium">{formatDate(w.start_time)}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">End</div>
                        <div className="text-sm font-medium">{formatDate(w.end_time)}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Suppress</div>
                        <div className="flex gap-2 mt-0.5">
                          {w.suppress_alerts && <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-warning/10 text-warning">Alerts</span>}
                          {w.suppress_notifications && <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-info/10 text-info">Notifs</span>}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-2 mt-3 pt-3 border-t border-outline-variant/10">
                    <button onClick={() => openEdit(w)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                    <button onClick={() => setDeleteTarget(w)} className="text-xs font-bold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </Card>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60" onClick={() => setShowForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20"><h2 className="font-headline text-lg font-bold">{editing ? 'Edit Window' : 'New Maintenance Window'}</h2></div>
            <div className="p-6 space-y-4 max-h-[60vh] overflow-y-auto">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Name</label>
                <input value={String(form.name ?? '')} onChange={(e) => setForm({ ...form, name: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. Sunday Lab Shutdown" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Schedule Type</label>
                  <select value={String(form.schedule_type ?? 'one_time')} onChange={(e) => setForm({ ...form, schedule_type: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                    <option value="one_time">One-time</option>
                    <option value="recurring">Recurring</option>
                  </select>
                </div>
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Scope</label>
                  <select value={String(form.scope_type ?? 'global')} onChange={(e) => setForm({ ...form, scope_type: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                    <option value="global">Global</option>
                    <option value="device">Device</option>
                    <option value="location">Location</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Start</label>
                  <input type="datetime-local" value={String(form.start_time ?? '').slice(0, 16)} onChange={(e) => setForm({ ...form, start_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary" />
                </div>
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">End</label>
                  <input type="datetime-local" value={String(form.end_time ?? '').slice(0, 16)} onChange={(e) => setForm({ ...form, end_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary" />
                </div>
              </div>
              {form.schedule_type === 'recurring' && (
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Recurrence Rule (iCal)</label>
                  <input value={String(form.recurrence_rule ?? '')} onChange={(e) => setForm({ ...form, recurrence_rule: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="FREQ=WEEKLY;BYDAY=SA" />
                </div>
              )}
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.suppress_alerts} onChange={(e) => setForm({ ...form, suppress_alerts: e.target.checked })} className="accent-primary" id="ma_alerts" />
                  <label htmlFor="ma_alerts" className="text-sm text-on-surface">Suppress alerts during window</label>
                </div>
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.suppress_notifications} onChange={(e) => setForm({ ...form, suppress_notifications: e.target.checked })} className="accent-primary" id="ma_notifs" />
                  <label htmlFor="ma_notifs" className="text-sm text-on-surface">Suppress notifications</label>
                </div>
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="accent-primary" id="ma_enabled" />
                  <label htmlFor="ma_enabled" className="text-sm text-on-surface">Enabled</label>
                </div>
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20">
              <button onClick={() => setShowForm(false)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleSubmit} disabled={submitting || !form.name} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editing ? 'Update' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete Window" message={`Delete "${deleteTarget?.name}"?`} confirmLabel="Delete" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
