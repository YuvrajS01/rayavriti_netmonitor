import { useState, useMemo } from 'react';
import type { ResourceRow } from '../api/resources';
import { useResourceList, useResourceMutations } from '../hooks/useResources';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { StatusPill } from '../components/ui/Status';
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
    <div className="space-y-6">
      <SectionHeader
        title="Maintenance"
        subtitle="Plan downtime windows that suppress alerts and notifications."
        action={<Button icon="add" onClick={openCreate}>New window</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total" value={stats.total} icon="event_repeat" />
        <StatCard label="Currently active" value={stats.active} icon="pending" color={stats.active ? 'text-status-maintenance' : undefined} />
        <StatCard label="Recurring" value={stats.recurring} icon="repeat" />
        <StatCard label="Suppress alerts" value={stats.suppress} icon="notifications_off" />
      </div>

      <GanttTimeline title="Maintenance schedule" items={windows.filter(window => window.start_time).map(window => ({ id: window.id, label: window.name, start: window.start_time, end: window.end_time, color: nowInRange(window.start_time, window.end_time) ? 'var(--color-status-maintenance)' : 'var(--color-chart-1)' }))} />

      <Card variant="low" className="overflow-hidden">
        <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
          <h2 className="text-base font-semibold">Maintenance windows <span className="text-sm font-normal text-fg-muted tabular-nums">({filtered.length})</span></h2>
          <div className="flex gap-3">
            <label>
              <span className="sr-only">Filter by schedule type</span>
              <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} className={INPUT_CLASS}>
                <option value="all">All types</option>
                <option value="one_time">One-time</option>
                <option value="recurring">Recurring</option>
              </select>
            </label>
            <label className="relative md:w-56">
              <span className="sr-only">Search windows</span>
              <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
              <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search windows" className={`${INPUT_CLASS} pl-10`} />
            </label>
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading…</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="event_repeat" title="No maintenance windows" description="Schedule downtime to suppress false alerts." action={<Button icon="add" onClick={openCreate}>Create window</Button>} />
        ) : (
          <ul className="divide-y divide-line">
            {filtered.map((w) => {
              const isActive = nowInRange(w.start_time, w.end_time);
              return (
                <li key={w.id} className="p-4 hover:bg-surface-2 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className={`w-10 h-10 rounded-md flex items-center justify-center flex-shrink-0 ${isActive ? 'bg-status-maintenance/12' : 'bg-surface-3'}`}>
                        <span className={`material-symbols-outlined text-xl ${isActive ? 'text-status-maintenance' : 'text-fg-muted'}`} aria-hidden="true">{w.schedule_type === 'recurring' ? 'repeat' : 'event'}</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="font-semibold truncate">{w.name}</h3>
                          {isActive && <StatusPill status="maintenance">Active now</StatusPill>}
                          {!w.enabled && <StatusPill status="unknown">Disabled</StatusPill>}
                        </div>
                        <p className="text-xs text-fg-muted mt-0.5"><span className="capitalize">{w.schedule_type?.replace('_', ' ')}</span> · {w.scope_type}: {w.scope_value}</p>
                        {w.description && <p className="text-sm text-fg-muted mt-1 line-clamp-1">{w.description}</p>}
                      </div>
                    </div>
                    <div className="flex items-start gap-4 flex-shrink-0">
                      <dl className="grid grid-cols-3 gap-4 lg:max-w-lg">
                        <div>
                          <dt className="text-xs text-fg-muted">Start</dt>
                          <dd className="text-sm font-medium">{formatDate(w.start_time)}</dd>
                        </div>
                        <div>
                          <dt className="text-xs text-fg-muted">End</dt>
                          <dd className="text-sm font-medium">{formatDate(w.end_time)}</dd>
                        </div>
                        <div>
                          <dt className="text-xs text-fg-muted">Suppresses</dt>
                          <dd className="text-sm font-medium">
                            {[w.suppress_alerts && 'Alerts', w.suppress_notifications && 'Notifications'].filter(Boolean).join(', ') || 'Nothing'}
                          </dd>
                        </div>
                      </dl>
                      <div className="flex gap-1">
                        <IconButton icon="edit" label={`Edit ${w.name}`} size="sm" onClick={() => openEdit(w)} />
                        <IconButton icon="delete" label={`Delete ${w.name}`} size="sm" variant="danger" onClick={() => setDeleteTarget(w)} />
                      </div>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Card>

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        closeOnBackdrop={false}
        title={editing ? 'Edit window' : 'New maintenance window'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button type="submit" form="maintenance-form" disabled={submitting || !form.name}>{submitting ? 'Saving…' : editing ? 'Save changes' : 'Create window'}</Button>
          </>
        }
      >
        <form id="maintenance-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
          <div>
            <label htmlFor="ma_name" className={LABEL_CLASS}>Name</label>
            <input id="ma_name" required value={asText(form.name)} onChange={(e) => setForm({ ...form, name: e.target.value })} className={INPUT_CLASS} placeholder="e.g. Sunday lab shutdown" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="ma_schedule" className={LABEL_CLASS}>Schedule type</label>
              <select id="ma_schedule" value={asText(form.schedule_type, 'one_time')} onChange={(e) => setForm({ ...form, schedule_type: e.target.value })} className={INPUT_CLASS}>
                <option value="one_time">One-time</option>
                <option value="recurring">Recurring</option>
              </select>
            </div>
            <div>
              <label htmlFor="ma_scope" className={LABEL_CLASS}>Scope</label>
              <select id="ma_scope" value={asText(form.scope_type, 'global')} onChange={(e) => setForm({ ...form, scope_type: e.target.value })} className={INPUT_CLASS}>
                <option value="global">Global</option>
                <option value="device">Device</option>
                <option value="location">Location</option>
              </select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="ma_start" className={LABEL_CLASS}>Start</label>
              <input id="ma_start" type="datetime-local" value={asText(form.start_time).slice(0, 16)} onChange={(e) => setForm({ ...form, start_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className={INPUT_CLASS} />
            </div>
            <div>
              <label htmlFor="ma_end" className={LABEL_CLASS}>End</label>
              <input id="ma_end" type="datetime-local" value={asText(form.end_time).slice(0, 16)} onChange={(e) => setForm({ ...form, end_time: e.target.value ? new Date(e.target.value).toISOString() : '' })} className={INPUT_CLASS} />
            </div>
          </div>
          {form.schedule_type === 'recurring' && (
            <div>
              <label htmlFor="ma_rrule" className={LABEL_CLASS}>Recurrence rule (iCal)</label>
              <input id="ma_rrule" value={asText(form.recurrence_rule)} onChange={(e) => setForm({ ...form, recurrence_rule: e.target.value })} className={`${INPUT_CLASS} font-mono`} placeholder="FREQ=WEEKLY;BYDAY=SA" />
            </div>
          )}
          <div className="space-y-2">
            {([
              ['suppress_alerts', 'Suppress alerts during window'],
              ['suppress_notifications', 'Suppress notifications'],
              ['enabled', 'Enabled'],
            ] as const).map(([key, label]) => (
              <label key={key} className="flex items-center gap-3 text-sm text-fg">
                <input type="checkbox" checked={!!form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.checked })} className="h-4 w-4 accent-primary" />
                {label}
              </label>
            ))}
          </div>
        </form>
      </Dialog>

      <ConfirmDialog open={!!deleteTarget} title="Delete window?" message={`Delete "${deleteTarget?.name}"?`} confirmLabel="Delete window" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const LABEL_CLASS = 'text-xs font-medium text-fg-muted block mb-1.5';
