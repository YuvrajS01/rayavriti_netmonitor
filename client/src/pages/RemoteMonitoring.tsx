import { useState, type FormEvent } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '../api/queryKeys';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import SectionHeader from '../components/ui/SectionHeader';
import EmptyState from '../components/ui/EmptyState';
import Dialog from '../components/ui/Dialog';
import ConfirmDialog from '../components/ConfirmDialog';
import { StatusPill } from '../components/ui/Status';
import { healthMeta } from '../utils/status';
import { createRemoteInstance, deleteRemoteInstance, getRemoteInstance, getRemoteInstances, getRemoteOverview, setRemoteMode, testRemoteInstance, type RemoteInput, type RemoteInstance, type RemoteOverview, type RemoteSnapshot } from '../api/remoteApi';

const emptyInput: RemoteInput = { name: '', url: '', apiKey: '', locationLabel: '', tags: [], pollIntervalS: 60, tlsSkipVerify: false };
const EMPTY_INSTANCES: RemoteInstance[] = [];
const EMPTY_OVERVIEW: RemoteOverview = { totalInstances: 0, online: 0, offline: 0, degraded: 0, alertCount: 0 };

function remoteInstancesFrom(value: unknown): RemoteInstance[] | null {
  if (Array.isArray(value)) return value as RemoteInstance[];
  if (value && typeof value === 'object') {
    const body = value as { instances?: unknown; items?: unknown };
    if (Array.isArray(body.instances)) return body.instances as RemoteInstance[];
    if (Array.isArray(body.items)) return body.items as RemoteInstance[];
  }
  return null;
}

function isRemoteMonitoringDisabled(reason: unknown): boolean {
  return (reason as { response?: { status?: unknown } })?.response?.status === 503;
}

export default function RemoteMonitoring() {
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<{ instance: RemoteInstance; snapshot: RemoteSnapshot | null } | null>(null);
  const [form, setForm] = useState<RemoteInput>(emptyInput);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actionError, setActionError] = useState('');
  const [removeTarget, setRemoveTarget] = useState<RemoteInstance | null>(null);

  // Invalidated on remote:status / remote:metrics events by useLiveQueryBridge.
  const overviewQuery = useQuery({ queryKey: queryKeys.remote.overview(), queryFn: ({ signal }) => getRemoteOverview({ signal }) });
  const instancesQuery = useQuery({ queryKey: queryKeys.remote.instances(), queryFn: ({ signal }) => getRemoteInstances({ signal }) });

  const remoteInstances = instancesQuery.data ? remoteInstancesFrom(instancesQuery.data.data) : null;
  const instances = remoteInstances ?? EMPTY_INSTANCES;
  const overview = overviewQuery.data?.data ?? EMPTY_OVERVIEW;
  const loadFailure = instancesQuery.error ?? overviewQuery.error;
  const loadError = loadFailure
    ? (isRemoteMonitoringDisabled(loadFailure)
        ? 'Remote monitoring is disabled on this server. Set REMOTE_ENABLED=true and restart the server.'
        : 'Unable to load remote monitoring data.')
    : instancesQuery.data && !remoteInstances
      ? 'Remote instances returned an unexpected response. Please try again.'
      : '';
  const error = actionError || loadError;

  const refresh = () => { void queryClient.invalidateQueries({ queryKey: queryKeys.remote.all }); };

  const select = (id: number) => { void getRemoteInstance(id).then((response) => setSelected(response.data)).catch(() => setActionError('Unable to load instance details.')); };
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setSaving(true); setActionError('');
    try { await createRemoteInstance(form); setShowForm(false); setForm(emptyInput); refresh(); }
    catch (reason) { setActionError(reason instanceof Error ? reason.message : 'Unable to register remote instance.'); }
    finally { setSaving(false); }
  };
  const test = async (id: number) => { try { await testRemoteInstance(id); refresh(); } catch { setActionError('Connection test failed. Check the URL and API key.'); } };
  const setMode = async (id: number, mode: 'active' | 'readonly' | 'maintenance') => { try { await setRemoteMode(id, mode); select(id); refresh(); } catch { setActionError('Unable to update service mode.'); } };
  const remove = async (id: number) => {
    setRemoveTarget(null);
    try { await deleteRemoteInstance(id); setSelected(null); refresh(); } catch { setActionError('Unable to remove remote instance.'); }
  };

  const snapshot = selected?.snapshot;

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Remote monitoring"
        subtitle="Monitor connected NetMonitor sites from one workspace."
        action={<Button icon="add_link" onClick={() => setShowForm(true)}>Add instance</Button>}
      />
      {error && (
        <div role="alert" className="flex items-start gap-3 rounded-md border border-line border-l-4 border-l-error bg-surface-1 px-4 py-3 text-sm text-fg">
          <span className="material-symbols-outlined text-[18px] text-error" aria-hidden="true">error</span>
          <span>{error}</span>
        </div>
      )}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <StatCard label="Instances" value={overview.totalInstances} icon="hub" />
        <StatCard label="Online" value={overview.online} icon="check_circle" />
        <StatCard label="Offline" value={overview.offline} color={overview.offline > 0 ? 'text-error' : undefined} icon="error" />
        <StatCard label="Degraded" value={overview.degraded} color={overview.degraded > 0 ? 'text-warning' : undefined} icon="warning" />
        <StatCard label="Active alerts" value={overview.alertCount} color={overview.alertCount > 0 ? 'text-error' : undefined} icon="notifications_active" />
      </div>
      {instances.length === 0 ? (
        <div className="rounded-lg border border-dashed border-line-strong bg-surface-1">
          <EmptyState icon="lan" title="No remote instances yet" description="Add a reachable NetMonitor instance and its API key to start aggregating health data." />
        </div>
      ) : (
        <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {instances.map((item) => (
            <li key={item.id}>
              <button type="button" onClick={() => select(item.id)} className="w-full rounded-lg border border-line bg-surface-1 p-5 text-left transition-colors hover:border-line-strong hover:bg-surface-2">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h2 className="text-lg font-semibold truncate">{item.name}</h2>
                    <p className="mt-1 truncate text-sm text-fg-muted">{item.locationLabel || item.url}</p>
                  </div>
                  <StatusPill status={item.status} />
                </div>
                <div className="mt-5 flex items-center justify-between gap-2 text-xs text-fg-muted">
                  <span>{item.locationLabel ? item.url : ''}</span>
                  <span className="tabular-nums">{item.lastSeenAt ? `Seen ${new Date(item.lastSeenAt).toLocaleTimeString()}` : 'Awaiting poll'}</span>
                </div>
                {item.lastError && <p className="mt-3 truncate text-xs text-error">{item.lastError}</p>}
              </button>
            </li>
          ))}
        </ul>
      )}

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        size="lg"
        closeOnBackdrop={false}
        title="Register remote instance"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button disabled={saving} type="submit" form="remote-form">{saving ? 'Saving…' : 'Register instance'}</Button>
          </>
        }
      >
        <form id="remote-form" onSubmit={submit} className="grid gap-4 sm:grid-cols-2">
          <label className={LABEL_CLASS}>Name<input required className={INPUT_CLASS} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></label>
          <label className={LABEL_CLASS}>Location<input className={INPUT_CLASS} value={form.locationLabel} onChange={(e) => setForm({ ...form, locationLabel: e.target.value })} /></label>
          <label className={`${LABEL_CLASS} sm:col-span-2`}>URL<input required type="url" placeholder="https://site.example.com" className={INPUT_CLASS} value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} /></label>
          <label className={`${LABEL_CLASS} sm:col-span-2`}>API key<input required type="password" autoComplete="off" className={INPUT_CLASS} value={form.apiKey} onChange={(e) => setForm({ ...form, apiKey: e.target.value })} /></label>
          <label className={LABEL_CLASS}>Poll interval (seconds)<input min="10" max="3600" type="number" className={INPUT_CLASS} value={form.pollIntervalS} onChange={(e) => setForm({ ...form, pollIntervalS: Number(e.target.value) })} /></label>
          <label className="flex items-end gap-2 pb-2 text-sm text-fg"><input type="checkbox" className="h-4 w-4 accent-primary" checked={form.tlsSkipVerify} onChange={(e) => setForm({ ...form, tlsSkipVerify: e.target.checked })} />Allow unverified TLS</label>
        </form>
      </Dialog>

      <Dialog
        open={!!selected}
        onClose={() => setSelected(null)}
        placement="drawer"
        title={selected?.instance.name ?? ''}
        description={selected?.instance.url}
        headerActions={selected ? <StatusPill status={selected.instance.status} /> : undefined}
        footer={
          selected ? (
            <>
              <Button variant="danger-outline" icon="delete" onClick={() => setRemoveTarget(selected.instance)}>Remove</Button>
              <Button variant="secondary" icon="wifi_tethering" onClick={() => test(selected.instance.id)}>Test connection</Button>
            </>
          ) : undefined
        }
      >
        {selected && (
          <div className="space-y-6">
            <dl className="grid grid-cols-2 gap-3">
              {[
                ['Devices', snapshot?.deviceCount ?? '—'],
                ['Up', snapshot?.deviceUpCount ?? '—'],
                ['Down', snapshot?.deviceDownCount ?? '—'],
                ['Alerts', snapshot?.alertCount ?? '—'],
                ['Latency', snapshot ? `${snapshot.latencyMs.toFixed(0)} ms` : '—'],
              ].map(([label, value]) => (
                <div key={String(label)} className="rounded-md border border-line bg-inset p-3">
                  <dt className="text-xs text-fg-muted">{label}</dt>
                  <dd className="mt-1 text-xl font-semibold tabular-nums">{value}</dd>
                </div>
              ))}
              <div className="rounded-md border border-line bg-inset p-3">
                <dt className="text-xs text-fg-muted">Health</dt>
                <dd className="mt-1 flex items-center gap-2">
                  <span className="text-xl font-semibold tabular-nums">{snapshot ? `${snapshot.healthScore.toFixed(0)}%` : '—'}</span>
                  {snapshot && <StatusPill status={healthMeta(snapshot.healthScore)} />}
                </dd>
              </div>
            </dl>
            <label className={LABEL_CLASS}>
              Service mode
              <select value={selected.instance.serviceMode || 'active'} onChange={(event) => setMode(selected.instance.id, event.target.value as 'active' | 'readonly' | 'maintenance')} className={INPUT_CLASS}>
                <option value="active">Active</option>
                <option value="readonly">Read-only</option>
                <option value="maintenance">Maintenance</option>
              </select>
            </label>
          </div>
        )}
      </Dialog>

      <ConfirmDialog
        open={!!removeTarget}
        title="Remove remote instance?"
        message={`Stop monitoring "${removeTarget?.name}" and delete its stored snapshots from this workspace?`}
        confirmLabel="Remove instance"
        danger
        onConfirm={() => { if (removeTarget) void remove(removeTarget.id); }}
        onCancel={() => setRemoveTarget(null)}
      />
    </div>
  );
}

const INPUT_CLASS = 'mt-1.5 w-full h-control rounded-md border border-line bg-inset px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const LABEL_CLASS = 'block text-xs font-medium text-fg-muted';
