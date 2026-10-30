import { useState, useMemo } from 'react';
import type { ResourceRow } from '../api/resources';
import { useResourceList, useResourceMutations } from '../hooks/useResources';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';
import GanttTimeline from '../components/charts/GanttTimeline';
import { asText } from '../utils/formatters';

interface MaintenanceWindow extends ResourceRow {
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

const EMPTY_WINDOWS: MaintenanceWindow[] = [];

export default function Maintenance() {
  const { addToast } = useToast();
  const windowsQuery = useResourceList<MaintenanceWindow>('/maintenance');
  const windows = windowsQuery.data ?? EMPTY_WINDOWS;
  const loading = windowsQuery.isPending;
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<MaintenanceWindow | null>(null);
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [deleteTarget, setDeleteTarget] = useState<MaintenanceWindow | null>(null);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return windows.filter((w) => {
      const matchSearch = !needle || w.name?.toLowerCase().includes(needle) || w.scope_value?.toLowerCase().includes(needle);
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

  const { save, remove } = useResourceMutations('/maintenance');
  const submitting = save.isPending;

  const handleSubmit = () => {
    const updating = editing != null;
    save.mutate({ id: editing?.id, body: form }, {
      onSuccess: () => {
        addToast(updating ? 'Window updated' : 'Window created', 'success');
        setShowForm(false);
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Save failed', 'error'),
    });
  };

  const handleDelete = () => {
    if (!deleteTarget) return;
    remove.mutate(deleteTarget.id, {
      onSuccess: () => {
        addToast('Window deleted', 'success');
        setDeleteTarget(null);
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
    });
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="Maintenance Calendar"
        subtitle="Plan downtime windows that suppress alerts and notifications."
        action={<Button icon="add" onClick={openCreate}>New Window</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Total" value={stats.total} icon="event_repeat" sparklineData={windows.map(window => window.enabled ? 1 : 0)} />
        <StatCard label="Currently active" value={stats.active} icon="pending" color="text-warning" trend={stats.active ? 'up' : 'flat'} sparklineData={windows.map(window => nowInRange(window.start_time, window.end_time) ? 1 : 0)} />
        <StatCard label="Recurring" value={stats.recurring} icon="repeat" sparklineData={windows.map(window => window.schedule_type === 'recurring' ? 1 : 0)} />
        <StatCard label="Suppress alerts" value={stats.suppress} icon="notifications_off" sparklineData={windows.map(window => window.suppress_alerts ? 1 : 0)} />
      </div>

      <GanttTimeline title="Maintenance schedule" items={windows.filter(window => window.start_time).map(window => ({ id: window.id, label: window.name, start: window.start_time, end: window.end_time, color: nowInRange(window.start_time, window.end_time) ? 'var(--color-warning)' : 'var(--color-info)' }))} />

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-line flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">event_repeat</span>
            </div>
            <div>
              <h2 className="font-sans font-semibold text-lg">Maintenance Windows</h2>
              <p className="text-xs text-fg-muted">{filtered.length} windows</p>
            </div>
          </div>
          <div className="flex gap-3">
            <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2.5 text-xs text-fg outline-none focus:ring-1 focus:ring-primary">
              <option value="all">All types</option>
              <option value="one_time">One-time</option>
              <option value="recurring">Recurring</option>
            </select>
            <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none md:w-56" />
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="event_repeat" title="No maintenance windows" description="Schedule downtime to suppress false alerts." action={<Button icon="add" onClick={openCreate}>Create Window</Button>} />
        ) : (
          <div className="divide-y divide-line">
            {filtered.map((w) => {
              const isActive = nowInRange(w.start_time, w.end_time);
              return (
                <article key={w.id} className="p-5 hover:bg-surface-1/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${isActive ? 'bg-warning/10' : 'bg-inset'}`}>
                        <span className={`material-symbols-outlined text-xl ${isActive ? 'text-warning' : 'text-fg-muted'}`}>{w.schedule_type === 'recurring' ? 'repeat' : 'event'}</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="font-sans font-semibold text-lg truncate">{w.name}</h3>
                          {isActive && <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-warning/10 text-warning">Active now</span>}
                          {!w.enabled && <span className="text-xs font-semibold px-2 py-0.5 rounded-full bg-line-strong/10 text-fg-subtle">Disabled</span>}
                        </div>
                        <p className="text-xs text-fg-muted mt-0.5 capitalize">{w.schedule_type?.replace('_', ' ')} · {w.scope_type}: {w.scope_value}</p>
                        {w.description && <p className="text-sm text-fg-muted mt-1 line-clamp-1">{w.description}</p>}
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 lg:max-w-lg flex-shrink-0">
                      <div>
                        <div className="text-xs text-fg-muted">Start</div>
                        <div className="text-sm font-medium">{formatDate(w.start_time)}</div>
                      </div>
                      <div>
                        <div className="text-xs text-fg-muted">End</div>
                        <div className="text-sm font-medium">{formatDate(w.end_time)}</div>
                      </div>
                      <div>
                        <div className="text-xs text-fg-muted">Suppress</div>
                        <div className="flex gap-2 mt-0.5">
                          {w.suppress_alerts && <span className="text-xs font-semibold px-1.5 py-0.5 rounded bg-warning/10 text-warning">Alerts</span>}
                          {w.suppress_notifications && <span className="text-xs font-semibold px-1.5 py-0.5 rounded bg-info/10 text-info">Notifs</span>}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-2 mt-3 pt-3 border-t border-line">
                    <button onClick={() => openEdit(w)} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                    <button onClick={() => setDeleteTarget(w)} className="text-xs font-semibold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </Card>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowForm(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">{editing ? 'Edit Window' : 'New Maintenance Window'}</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-xs text-fg-muted block mb-1">Name</label>
                <input value={asText(form.name)} onChange={(e) => setForm({ ...form, name: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. Sunday Lab Shutdown" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-fg-muted block mb-1">Schedule type</label>
                  <select value={asText(form.schedule_type, 'one_time')} onChange={(e) => setForm({ ...form, schedule_type: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                    <option value="one_time">One-time</option>
                    <option value="recurring">Recurring</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs text-fg-muted block mb-1">Scope</label>
                  <select value={asText(form.scope_type, 'global')} onChange={(e) => setForm({ ...form, scope_type: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                    <option value="global">Global</option>
                    <option value="device">Device</option>
                    <option value="location">Location</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-xs text-fg-muted block mb-1">Start</label>
                  <input type="datetime-local" value={asText(form.start_time).slice(0, 16)} onChange={(e) => setForm({ ...form, start_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary" />
                </div>
                <div>
                  <label className="text-xs text-fg-muted block mb-1">End</label>
                  <input type="datetime-local" value={asText(form.end_time).slice(0, 16)} onChange={(e) => setForm({ ...form, end_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary" />
                </div>
              </div>
              {form.schedule_type === 'recurring' && (
                <div>
                  <label className="text-xs text-fg-muted block mb-1">Recurrence rule (iCal)</label>
                  <input value={asText(form.recurrence_rule)} onChange={(e) => setForm({ ...form, recurrence_rule: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="FREQ=WEEKLY;BYDAY=SA" />
                </div>
              )}
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.suppress_alerts} onChange={(e) => setForm({ ...form, suppress_alerts: e.target.checked })} className="accent-primary" id="ma_alerts" />
                  <label htmlFor="ma_alerts" className="text-sm text-fg">Suppress alerts during window</label>
                </div>
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.suppress_notifications} onChange={(e) => setForm({ ...form, suppress_notifications: e.target.checked })} className="accent-primary" id="ma_notifs" />
                  <label htmlFor="ma_notifs" className="text-sm text-fg">Suppress notifications</label>
                </div>
                <div className="flex items-center gap-3">
                  <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="accent-primary" id="ma_enabled" />
                  <label htmlFor="ma_enabled" className="text-sm text-fg">Enabled</label>
                </div>
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowForm(false)} className="flex-1 py-3 text-xs font-sans font-semibold text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleSubmit} disabled={submitting || !form.name} className="flex-1 py-3 text-xs font-sans font-semibold text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editing ? 'Update' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete Window" message={`Delete "${deleteTarget?.name}"?`} confirmLabel="Delete" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
