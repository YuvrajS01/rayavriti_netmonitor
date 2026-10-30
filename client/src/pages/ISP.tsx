import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createISPLink, deleteISPLink, getISPComparison, getISPLinks, updateISPLink, type ISPCompareLink, type ISPComparison, type ISPLink } from '../api/isp';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';
import ISPLinkModal from '../components/ISPLinkModal';

const ISP_POLL_MS = 15_000;
const EMPTY_LINKS: ISPLink[] = [];

export default function ISP() {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<ISPLink | null>(null);
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [deleteTarget, setDeleteTarget] = useState<ISPLink | null>(null);
  const [selectedLink, setSelectedLink] = useState<ISPLink | null>(null);

  // Probe results change continuously; poll at the cadence the old
  // socket-throttled refresh used (paused in background tabs).
  const linksQuery = useQuery({
    queryKey: queryKeys.isp.links(),
    queryFn: ({ signal }) => getISPLinks({ signal }),
    refetchInterval: ISP_POLL_MS,
  });
  const comparisonQuery = useQuery({
    queryKey: queryKeys.isp.comparison(),
    queryFn: ({ signal }) => getISPComparison({ signal }),
    refetchInterval: ISP_POLL_MS,
  });
  const loading = linksQuery.isPending;
  const links = linksQuery.data?.data || EMPTY_LINKS;
  const comparison: ISPComparison | null = comparisonQuery.data?.data ?? null;

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.isp.all });

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return links.filter((l) => !needle || l.name.toLowerCase().includes(needle) || l.provider.toLowerCase().includes(needle));
  }, [links, search]);

  const compMap = useMemo(() => {
    const m = new Map<number, ISPCompareLink>();
    comparison?.links?.forEach((l) => m.set(l.id, l));
    return m;
  }, [comparison]);

  const stats = useMemo(() => ({
    total: links.length,
    enabled: links.filter((l) => l.enabled).length,
    avgBandwidth: links.length ? Math.round(links.reduce((s, l) => s + l.bandwidth_mbps, 0) / links.length) : 0,
    totalBandwidth: links.reduce((s, l) => s + l.bandwidth_mbps, 0),
  }), [links]);

  const openCreate = () => { setEditing(null); setForm({ name: '', provider: '', gateway_ip: '', bandwidth_mbps: 100, sla_uptime_percent: 99.5, monitoring_interval_seconds: 10, enabled: true }); setShowForm(true); };
  const openEdit = (l: ISPLink) => { setEditing(l); setForm({ ...l }); setShowForm(true); };

  const saveMutation = useMutation({
    mutationFn: ({ target, body }: { target: ISPLink | null; body: Record<string, unknown> }) =>
      target ? updateISPLink(target.id, body) : createISPLink(body),
    onSuccess: (_res, { target }) => {
      addToast(target ? 'Link updated' : 'Link created', 'success');
      setShowForm(false);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Save failed', 'error'),
  });
  const submitting = saveMutation.isPending;

  const deleteMutation = useMutation({
    mutationFn: (link: ISPLink) => deleteISPLink(link.id),
    onSuccess: () => {
      addToast('Link deleted', 'success');
      setDeleteTarget(null);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
  });

  const handleSubmit = () => saveMutation.mutate({ target: editing, body: form });

  const handleDelete = () => {
    if (deleteTarget) deleteMutation.mutate(deleteTarget);
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="ISP Dashboard"
        subtitle="Monitor provider circuits, bandwidth usage, and SLA compliance."
        action={<Button icon="add" onClick={openCreate}>Add Link</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Total Links" value={stats.total} icon="router" sparklineData={links.map(link => link.bandwidth_mbps)} />
        <StatCard label="Enabled" value={stats.enabled} icon="check_circle" trend="up" sparklineData={links.map(link => link.enabled ? 1 : 0)} />
        <StatCard label="Total Bandwidth" value={`${stats.totalBandwidth} Mbps`} icon="speed" sparklineData={links.map(link => link.bandwidth_mbps)} />
        <StatCard label="Avg Bandwidth" value={`${stats.avgBandwidth} Mbps`} icon="data_usage" sparklineData={links.map(link => link.bandwidth_mbps)} />
      </div>

      {comparison?.links?.length ? <Card variant="low" className="p-5"><div className="flex items-center justify-between mb-4"><div><h2 className="font-headline text-sm font-semibold uppercase tracking-wide">Circuit throughput comparison</h2><p className="text-xs text-on-surface-variant mt-1">Download and upload against provisioned capacity</p></div><span className="material-symbols-outlined text-info">speed</span></div><div className="space-y-4">{comparison.links.map(link => <div key={link.id} className="grid grid-cols-[minmax(90px,160px)_1fr_auto] gap-4 items-center"><div className="min-w-0"><strong className="text-sm block truncate">{link.name}</strong><span className="text-[10px] uppercase tracking-wide text-on-surface-variant">{link.provider}</span></div><div className="space-y-1.5"><div className="h-2 rounded-full bg-surface-container-lowest overflow-hidden"><i className="block h-full bg-info rounded-full" style={{ width: `${Math.min(100, link.bandwidthMbps ? link.avgDownload / link.bandwidthMbps * 100 : 0)}%` }} /></div><div className="h-1 rounded-full bg-surface-container-lowest overflow-hidden"><i className="block h-full bg-primary rounded-full" style={{ width: `${Math.min(100, link.bandwidthMbps ? link.avgUpload / link.bandwidthMbps * 100 : 0)}%` }} /></div></div><span className="font-data text-xs text-on-surface-variant">↓{link.avgDownload.toFixed(1)} ↑{link.avgUpload.toFixed(1)} Mbps</span></div>)}</div></Card> : null}

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">router</span>
            </div>
            <div>
              <h2 className="font-headline font-semibold text-lg">ISP Links</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{filtered.length} links</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search links..." className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="router" title="No ISP links" description="Add provider circuits to start monitoring." action={<Button icon="add" onClick={openCreate}>Add Link</Button>} />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filtered.map((link) => {
              const comp = compMap.get(link.id);
              return (
                <article key={link.id} className="p-5 hover:bg-surface-container-low/50 transition-colors cursor-pointer" onClick={() => setSelectedLink(link)}>
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-lg bg-surface-container-lowest flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">cell_tower</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-headline font-semibold text-lg truncate">{link.name}</h3>
                          {comp && <span className={`text-xs font-semibold uppercase px-2 py-0.5 rounded-full ${comp.status === 'up' ? 'bg-success/10 text-success' : comp.status === 'degraded' ? 'bg-warning/10 text-warning' : 'bg-error/10 text-error'}`}>{comp.status}</span>}
                        </div>
                        <p className="text-xs text-on-surface-variant mt-0.5">{link.provider} · {link.gateway_ip} · #{link.id}</p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:max-w-2xl flex-shrink-0">
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Bandwidth</div>
                        <div className="text-sm font-medium">{link.bandwidth_mbps} Mbps</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Latency</div>
                        <div className={`text-sm font-medium ${comp && comp.avgLatencyMs > 50 ? 'text-warning' : ''}`}>{comp ? `${comp.avgLatencyMs.toFixed(1)}ms` : '-'}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Packet Loss</div>
                        <div className={`text-sm font-medium ${comp && comp.avgPacketLoss > 1 ? 'text-error' : 'text-success'}`}>{comp ? `${comp.avgPacketLoss.toFixed(2)}%` : '-'}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">SLA</div>
                        <div className={`text-sm font-medium ${comp && comp.uptimePercent >= (link.sla_uptime_percent || 99.5) ? 'text-success' : 'text-error'}`}>
                          {comp ? `${comp.uptimePercent.toFixed(1)}%` : '-'} / {link.sla_uptime_percent || 99.5}%
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-2 mt-3 pt-3 border-t border-outline-variant/10" onClick={(e) => e.stopPropagation()}>
                    <button onClick={() => openEdit(link)} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                    <button onClick={() => setDeleteTarget(link)} className="text-xs font-semibold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </Card>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0"><h2 className="font-headline text-lg font-semibold">{editing ? 'Edit Link' : 'New ISP Link'}</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              {([
                { key: 'name', label: 'Name', type: 'text' },
                { key: 'provider', label: 'Provider', type: 'text' },
                { key: 'gateway_ip', label: 'Gateway IP', type: 'text' },
                { key: 'bandwidth_mbps', label: 'Bandwidth (Mbps)', type: 'number' },
                { key: 'sla_uptime_percent', label: 'SLA Uptime %', type: 'number' },
                { key: 'monitoring_interval_seconds', label: 'Probe Interval (s)', type: 'number' },
              ] as const).map(({ key, label, type }) => (
                <div key={key}>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">{label}</label>
                  <input type={type} value={String(form[key] ?? '')} onChange={(e) => setForm({ ...form, [key]: type === 'number' ? Number(e.target.value) : e.target.value })} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" />
                </div>
              ))}
              <div className="flex items-center gap-3">
                <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="accent-primary" id="isp_enabled" />
                <label htmlFor="isp_enabled" className="text-sm text-on-surface">Enabled</label>
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowForm(false)} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-low transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleSubmit} disabled={submitting || !form.name} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editing ? 'Update' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete ISP Link" message={`Delete "${deleteTarget?.name}"? Monitoring data will be lost.`} confirmLabel="Delete" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />

      {selectedLink && <ISPLinkModal link={selectedLink} onClose={() => setSelectedLink(null)} />}
    </div>
  );
}
