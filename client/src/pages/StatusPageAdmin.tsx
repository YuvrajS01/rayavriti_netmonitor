import { useState, useEffect, useMemo } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface StatusService {
  id: number;
  name: string;
  description: string;
  group_name: string;
  aggregation: string;
  display_order: number;
  show_response_time: boolean;
  show_uptime: boolean;
  enabled: boolean;
}

interface StatusIncident {
  id: number;
  title: string;
  message: string;
  severity: string;
  status: string;
  started_at: string;
  resolved_at: string | null;
}

export default function StatusPageAdmin() {
  const { addToast } = useToast();
  const [services, setServices] = useState<StatusService[]>([]);
  const [incidents, setIncidents] = useState<StatusIncident[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<'services' | 'incidents'>('services');
  const [search, setSearch] = useState('');
  const [showServiceForm, setShowServiceForm] = useState(false);
  const [editingService, setEditingService] = useState<StatusService | null>(null);
  const [serviceForm, setServiceForm] = useState<Record<string, unknown>>({});
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<StatusService | null>(null);

  const load = async () => {
    setLoading(true);
    const [svcRes, incRes] = await Promise.all([
      v1.get('/status-page/services'),
      v1.get('/status-page/incidents'),
    ]);
    setServices(wrap<StatusService[]>(svcRes.data).data || []);
    setIncidents(wrap<StatusIncident[]>(incRes.data).data || []);
    setLoading(false);
  };

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const [svcRes, incRes] = await Promise.all([
          v1.get('/status-page/services'),
          v1.get('/status-page/incidents'),
        ]);
        if (active) setServices(wrap<StatusService[]>(svcRes.data).data || []);
        if (active) setIncidents(wrap<StatusIncident[]>(incRes.data).data || []);
      } catch {
        if (active) {
          setServices([]);
          setIncidents([]);
        }
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, []);

  const filteredServices = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return services.filter((s) => !needle || s.name.toLowerCase().includes(needle) || s.group_name?.toLowerCase().includes(needle));
  }, [services, search]);

  const filteredIncidents = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return incidents.filter((i) => !needle || i.title.toLowerCase().includes(needle));
  }, [incidents, search]);

  const stats = useMemo(() => ({
    totalServices: services.length,
    enabledServices: services.filter((s) => s.enabled).length,
    activeIncidents: incidents.filter((i) => !i.resolved_at).length,
    groups: new Set(services.map((s) => s.group_name).filter(Boolean)).size,
  }), [services, incidents]);

  const openCreateService = () => {
    setEditingService(null);
    setServiceForm({ name: '', description: '', group_name: 'General', aggregation: 'any_down', show_uptime: true, show_response_time: false, enabled: true, display_order: 0 });
    setShowServiceForm(true);
  };
  const openEditService = (s: StatusService) => { setEditingService(s); setServiceForm({ ...s }); setShowServiceForm(true); };

  const handleServiceSubmit = async () => {
    setSubmitting(true);
    try {
      if (editingService) {
        await v1.put(`/status-page/services/${editingService.id}`, serviceForm);
        addToast('Service updated', 'success');
      } else {
        await v1.post('/status-page/services', serviceForm);
        addToast('Service created', 'success');
      }
      setShowServiceForm(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Save failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteService = async () => {
    if (!deleteTarget) return;
    try {
      await v1.delete(`/status-page/services/${deleteTarget.id}`);
      addToast('Service deleted', 'success');
      setDeleteTarget(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="Status Page Admin"
        subtitle="Configure public services and incident announcements."
        action={tab === 'services' && <Button icon="add" onClick={openCreateService}>Add Service</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Services" value={stats.totalServices} icon="dns" />
        <StatCard label="Active Incidents" value={stats.activeIncidents} icon="crisis_alert" color={stats.activeIncidents > 0 ? 'text-error' : undefined} />
        <StatCard label="Groups" value={stats.groups} icon="folder" />
        <StatCard label="Enabled" value={stats.enabledServices} icon="check_circle" />
      </div>

      <div className="flex gap-1 bg-surface-container-low rounded-lg p-1 w-fit">
        {(['services', 'incidents'] as const).map((t) => (
          <button key={t} onClick={() => { setTab(t); setSearch(''); }} className={`px-4 py-2 text-xs font-headline font-semibold uppercase tracking-wide rounded-md transition-colors ${tab === t ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:text-on-surface'}`}>
            {t === 'services' ? 'Services' : 'Incidents'}
          </button>
        ))}
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">{tab === 'services' ? 'dns' : 'crisis_alert'}</span>
            </div>
            <div>
              <h2 className="font-headline font-semibold text-lg">{tab === 'services' ? 'Services' : 'Incidents'}</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{tab === 'services' ? filteredServices.length : filteredIncidents.length} items</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : tab === 'services' ? (
          filteredServices.length === 0 ? (
            <EmptyState icon="dns" title="No services configured" description="Add services to display on the public status page." action={<Button icon="add" onClick={openCreateService}>Add Service</Button>} />
          ) : (
            <div className="divide-y divide-outline-variant/20">
              {filteredServices.map((s) => (
                <article key={s.id} className="p-5 hover:bg-surface-container-low/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                    <div className="flex items-center gap-4 min-w-0">
                      <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${s.enabled ? 'bg-success/10' : 'bg-surface-container-lowest'}`}>
                        <span className={`material-symbols-outlined text-xl ${s.enabled ? 'text-success' : 'text-outline'}`}>{s.enabled ? 'check_circle' : 'radio_button_unchecked'}</span>
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-headline font-semibold text-lg truncate">{s.name}</h3>
                        <p className="text-xs text-on-surface-variant mt-0.5">{s.group_name || 'Ungrouped'} · {s.aggregation?.replace('_', ' ')}</p>
                        {s.description && <p className="text-sm text-on-surface-variant mt-1 line-clamp-1">{s.description}</p>}
                      </div>
                    </div>
                    <div className="flex gap-2 items-center">
                      {s.show_uptime && <span className="text-[10px] font-semibold px-2 py-0.5 rounded bg-primary/10 text-primary">Uptime</span>}
                      {s.show_response_time && <span className="text-[10px] font-semibold px-2 py-0.5 rounded bg-info/10 text-info">Response</span>}
                      <button onClick={() => openEditService(s)} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                      <button onClick={() => setDeleteTarget(s)} className="text-xs font-semibold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )
        ) : filteredIncidents.length === 0 ? (
          <EmptyState icon="crisis_alert" title="No public incidents" description="Active incidents will appear here." />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filteredIncidents.map((i) => (
              <article key={i.id} className="p-5 hover:bg-surface-container-low/50 transition-colors">
                <div className="flex items-start gap-4">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${i.resolved_at ? 'bg-success/10' : i.severity === 'critical' ? 'bg-error/10' : 'bg-warning/10'}`}>
                    <span className={`material-symbols-outlined text-xl ${i.resolved_at ? 'text-success' : i.severity === 'critical' ? 'text-error' : 'text-warning'}`}>{i.resolved_at ? 'check_circle' : 'crisis_alert'}</span>
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <h3 className="font-headline font-semibold text-lg">{i.title}</h3>
                      <span className={`text-[10px] font-semibold uppercase px-2 py-0.5 rounded-full ${i.resolved_at ? 'bg-success/10 text-success' : 'bg-error/10 text-error'}`}>{i.resolved_at ? 'resolved' : i.status}</span>
                    </div>
                    <p className="text-xs text-on-surface-variant mt-0.5 capitalize">{i.severity} · #{i.id}</p>
                    {i.message && <p className="text-sm text-on-surface-variant mt-2 line-clamp-2">{i.message}</p>}
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {showServiceForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowServiceForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0"><h2 className="font-headline text-lg font-semibold">{editingService ? 'Edit Service' : 'New Service'}</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              {[
                { key: 'name', label: 'Service Name', type: 'text' },
                { key: 'description', label: 'Description', type: 'text' },
                { key: 'group_name', label: 'Group', type: 'text' },
              ].map(({ key, label }) => (
                <div key={key}>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">{label}</label>
                  <input value={String(serviceForm[key] ?? '')} onChange={(e) => setServiceForm({ ...serviceForm, [key]: e.target.value })} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" />
                </div>
              ))}
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Aggregation</label>
                <select value={String(serviceForm.aggregation ?? 'any_down')} onChange={(e) => setServiceForm({ ...serviceForm, aggregation: e.target.value })} className="w-full bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                  <option value="any_down">Any Down</option>
                  <option value="all_down">All Down</option>
                  <option value="worst">Worst Status</option>
                </select>
              </div>
              <div className="space-y-2">
                {[
                  { key: 'show_uptime', label: 'Show uptime percentage' },
                  { key: 'show_response_time', label: 'Show response time' },
                  { key: 'enabled', label: 'Enabled' },
                ].map(({ key, label }) => (
                  <div key={key} className="flex items-center gap-3">
                    <input type="checkbox" checked={!!serviceForm[key]} onChange={(e) => setServiceForm({ ...serviceForm, [key]: e.target.checked })} className="accent-primary" id={`sp_${key}`} />
                    <label htmlFor={`sp_${key}`} className="text-sm text-on-surface">{label}</label>
                  </div>
                ))}
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowServiceForm(false)} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-low transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleServiceSubmit} disabled={submitting || !serviceForm.name} className="flex-1 py-3 text-xs font-headline font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editingService ? 'Update' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete Service" message={`Remove "${deleteTarget?.name}" from the status page?`} confirmLabel="Delete" danger onConfirm={handleDeleteService} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
