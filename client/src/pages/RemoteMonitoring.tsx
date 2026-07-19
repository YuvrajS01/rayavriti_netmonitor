import { useCallback, useEffect, useState, type FormEvent } from 'react';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import { useSocketContext } from '../hooks/useSocket';
import { createRemoteInstance, deleteRemoteInstance, getRemoteInstance, getRemoteInstances, getRemoteOverview, setRemoteMode, testRemoteInstance, type RemoteInput, type RemoteInstance, type RemoteOverview, type RemoteSnapshot } from '../api/remoteApi';

const emptyInput: RemoteInput = { name: '', url: '', apiKey: '', locationLabel: '', tags: [], pollIntervalS: 60, tlsSkipVerify: false };
const statusStyle: Record<string, string> = { online: 'bg-success', offline: 'bg-error', degraded: 'bg-warning', unknown: 'bg-outline' };

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
  const { subscribe } = useSocketContext();
  const [overview, setOverview] = useState<RemoteOverview>({ totalInstances: 0, online: 0, offline: 0, degraded: 0, alertCount: 0 });
  const [instances, setInstances] = useState<RemoteInstance[]>([]);
  const [selected, setSelected] = useState<{ instance: RemoteInstance; snapshot: RemoteSnapshot | null } | null>(null);
  const [form, setForm] = useState<RemoteInput>(emptyInput);
  const [showForm, setShowForm] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const refresh = useCallback(() => {
    void Promise.all([getRemoteOverview(), getRemoteInstances()])
      .then(([overviewResponse, instancesResponse]) => {
        const remoteInstances = remoteInstancesFrom(instancesResponse.data);
        if (!remoteInstances) {
          setInstances([]);
          setError('Remote instances returned an unexpected response. Please try again.');
          return;
        }
        setOverview(overviewResponse.data);
        setInstances(remoteInstances);
        setError('');
      })
      .catch((reason) => setError(isRemoteMonitoringDisabled(reason)
        ? 'Remote monitoring is disabled on this server. Set REMOTE_ENABLED=true and restart the server.'
        : 'Unable to load remote monitoring data.'));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => subscribe('remote:status' as never, refresh), [subscribe, refresh]);
  useEffect(() => subscribe('remote:metrics' as never, refresh), [subscribe, refresh]);

  const select = (id: number) => { void getRemoteInstance(id).then((response) => setSelected(response.data)).catch(() => setError('Unable to load instance details.')); };
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setSaving(true); setError('');
    try { await createRemoteInstance(form); setShowForm(false); setForm(emptyInput); refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Unable to register remote instance.'); }
    finally { setSaving(false); }
  };
  const test = async (id: number) => { try { await testRemoteInstance(id); refresh(); } catch { setError('Connection test failed. Check the URL and API key.'); } };
  const setMode = async (id: number, mode: 'active' | 'readonly' | 'maintenance') => { try { await setRemoteMode(id, mode); select(id); refresh(); } catch { setError('Unable to update service mode.'); } };
  const remove = async (id: number) => { if (!window.confirm('Remove this remote instance?')) return; try { await deleteRemoteInstance(id); setSelected(null); refresh(); } catch { setError('Unable to remove remote instance.'); } };

  return <div className="mx-auto max-w-7xl space-y-6 p-4 sm:p-6">
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div><p className="text-xs font-label uppercase tracking-widest text-primary">Fleet Operations</p><h1 className="font-headline text-3xl font-semibold">Remote Monitoring</h1><p className="mt-1 text-sm text-on-surface-variant">Monitor connected NetMonitor sites from one workspace.</p></div>
      <Button icon="add_link" onClick={() => setShowForm(true)}>Add instance</Button>
    </div>
    {error && <div className="rounded-md border border-error/30 bg-error-container/40 px-4 py-3 text-sm text-on-error-container">{error}</div>}
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-5"><StatCard label="Instances" value={overview.totalInstances} icon="hub" sparklineData={[overview.totalInstances, instances.length, overview.totalInstances]} /><StatCard label="Online" value={overview.online} color="text-success" icon="check_circle" trend="up" sparklineData={[overview.online, overview.totalInstances, overview.online]} /><StatCard label="Offline" value={overview.offline} color="text-error" icon="error" trend="down" sparklineData={[overview.offline, overview.degraded, overview.offline]} /><StatCard label="Degraded" value={overview.degraded} color="text-warning" icon="warning" sparklineData={[overview.degraded, overview.online, overview.degraded]} /><StatCard label="Active alerts" value={overview.alertCount} color="text-error" icon="notifications_active" sparklineData={[overview.alertCount, overview.offline, overview.alertCount]} /></div>
    {instances.length === 0 ? <div className="rounded-lg border border-dashed border-outline-variant/50 bg-surface-container-low p-12 text-center"><span className="material-symbols-outlined text-4xl text-on-surface-variant">lan</span><h2 className="mt-3 font-headline text-lg font-semibold">No remote instances yet</h2><p className="mt-1 text-sm text-on-surface-variant">Add a reachable NetMonitor instance and its API key to start aggregating health data.</p></div> : <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{instances.map((item) => <button key={item.id} onClick={() => select(item.id)} className="rounded-lg border border-outline-variant/30 bg-surface-container-low p-5 text-left transition hover:border-primary/50 hover:bg-surface-container">
      <div className="flex items-start justify-between gap-3"><div><h2 className="font-headline text-lg font-semibold">{item.name}</h2><p className="mt-1 truncate text-sm text-on-surface-variant">{item.locationLabel || item.url}</p></div><span className={`mt-1 h-3 w-3 shrink-0 rounded-full ${statusStyle[item.status] || statusStyle.unknown}`} title={item.status} /></div>
      <div className="mt-5 flex items-center justify-between text-xs text-on-surface-variant"><span className="capitalize">{item.status}</span><span>{item.lastSeenAt ? `Seen ${new Date(item.lastSeenAt).toLocaleTimeString()}` : 'Awaiting poll'}</span></div>
      {item.lastError && <p className="mt-3 truncate text-xs text-error">{item.lastError}</p>}
    </button>)}</div>}
    {showForm && <div className="fixed inset-0 z-50 grid place-items-center bg-scrim/60 p-4"><form onSubmit={submit} className="w-full max-w-xl rounded-xl bg-surface p-6 shadow-2xl"><div className="flex items-center justify-between"><h2 className="font-headline text-xl font-semibold">Register remote instance</h2><button type="button" className="material-symbols-outlined" onClick={() => setShowForm(false)}>close</button></div><div className="mt-5 grid gap-4 sm:grid-cols-2">
      <label className="text-sm">Name<input required className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></label>
      <label className="text-sm">Location<label className="sr-only">Location</label><input className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2" value={form.locationLabel} onChange={(e) => setForm({ ...form, locationLabel: e.target.value })} /></label>
      <label className="text-sm sm:col-span-2">URL<input required type="url" placeholder="https://site.example.com" className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} /></label>
      <label className="text-sm sm:col-span-2">API key<input required type="password" className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2" value={form.apiKey} onChange={(e) => setForm({ ...form, apiKey: e.target.value })} /></label>
      <label className="text-sm">Poll interval (seconds)<input min="10" max="3600" type="number" className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2" value={form.pollIntervalS} onChange={(e) => setForm({ ...form, pollIntervalS: Number(e.target.value) })} /></label>
      <label className="flex items-end gap-2 pb-2 text-sm"><input type="checkbox" checked={form.tlsSkipVerify} onChange={(e) => setForm({ ...form, tlsSkipVerify: e.target.checked })} />Allow unverified TLS</label>
    </div><div className="mt-6 flex justify-end gap-3"><Button variant="ghost" type="button" onClick={() => setShowForm(false)}>Cancel</Button><Button disabled={saving} type="submit">{saving ? 'Saving…' : 'Register instance'}</Button></div></form></div>}
    {selected && <div className="fixed inset-0 z-50 flex justify-end bg-scrim/40" onClick={() => setSelected(null)}><aside className="h-full w-full max-w-lg overflow-y-auto bg-surface p-6 shadow-2xl" onClick={(event) => event.stopPropagation()}><div className="flex items-start justify-between"><div><h2 className="font-headline text-2xl font-semibold">{selected.instance.name}</h2><p className="mt-1 text-sm text-on-surface-variant">{selected.instance.url}</p></div><button className="material-symbols-outlined" onClick={() => setSelected(null)}>close</button></div><div className="mt-6 grid grid-cols-2 gap-3">{[['Devices', selected.snapshot?.deviceCount ?? '—'], ['Up', selected.snapshot?.deviceUpCount ?? '—'], ['Down', selected.snapshot?.deviceDownCount ?? '—'], ['Alerts', selected.snapshot?.alertCount ?? '—'], ['Health', selected.snapshot ? `${selected.snapshot.healthScore.toFixed(0)}%` : '—'], ['Latency', selected.snapshot ? `${selected.snapshot.latencyMs.toFixed(0)} ms` : '—']].map(([label, value]) => <StatCard key={String(label)} label={String(label)} value={String(value)} />)}</div><label className="mt-6 block text-sm font-medium">Service mode<select value={selected.instance.serviceMode || 'active'} onChange={(event) => setMode(selected.instance.id, event.target.value as 'active' | 'readonly' | 'maintenance')} className="mt-1 w-full rounded-md border border-outline-variant bg-surface-container-low px-3 py-2"><option value="active">Active</option><option value="readonly">Read-only</option><option value="maintenance">Maintenance</option></select></label><div className="mt-6 flex gap-3"><Button variant="primary-outline" icon="wifi_tethering" onClick={() => test(selected.instance.id)}>Test connection</Button><Button variant="danger-outline" icon="delete" onClick={() => remove(selected.instance.id)}>Remove</Button></div></aside></div>}
  </div>;
}
