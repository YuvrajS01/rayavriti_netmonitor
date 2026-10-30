import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createStatusService, deleteStatusService, getStatusIncidents, getStatusServices, updateStatusService, type StatusIncident, type StatusService } from '../api/statusPage';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { Tabs, TabPanel } from '../components/ui/Tabs';
import { StatusPill } from '../components/ui/Status';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { lifecycleMeta, severityMeta } from '../utils/status';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';

const STATUS_TABS = ['services', 'incidents'] as const;
type StatusTab = (typeof STATUS_TABS)[number];

const EMPTY_SERVICES: StatusService[] = [];
const EMPTY_INCIDENTS: StatusIncident[] = [];

export default function StatusPageAdmin() {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [tab, setTab] = useTabParam<StatusTab>(STATUS_TABS);
  const tabsId = useTabsId();
  const [search, setSearch] = useState('');
  const [showServiceForm, setShowServiceForm] = useState(false);
  const [editingService, setEditingService] = useState<StatusService | null>(null);
  const [serviceForm, setServiceForm] = useState<Record<string, unknown>>({});
  const [deleteTarget, setDeleteTarget] = useState<StatusService | null>(null);

  const servicesQuery = useQuery({ queryKey: queryKeys.statusPage.services(), queryFn: ({ signal }) => getStatusServices({ signal }) });
  const incidentsQuery = useQuery({ queryKey: queryKeys.statusPage.incidents(), queryFn: ({ signal }) => getStatusIncidents({ signal }) });
  const loading = servicesQuery.isPending || incidentsQuery.isPending;
  const services = servicesQuery.data?.data || EMPTY_SERVICES;
  const incidents = incidentsQuery.data?.data || EMPTY_INCIDENTS;

  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.statusPage.all });

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

  const saveMutation = useMutation({
    mutationFn: ({ target, body }: { target: StatusService | null; body: Record<string, unknown> }) =>
      target ? updateStatusService(target.id, body) : createStatusService(body),
    onSuccess: (_res, { target }) => {
      addToast(target ? 'Service updated' : 'Service created', 'success');
      setShowServiceForm(false);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Save failed', 'error'),
  });
  const submitting = saveMutation.isPending;

  const deleteMutation = useMutation({
    mutationFn: (service: StatusService) => deleteStatusService(service.id),
    onSuccess: () => {
      addToast('Service deleted', 'success');
      setDeleteTarget(null);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
  });

  const handleServiceSubmit = () => saveMutation.mutate({ target: editingService, body: serviceForm });

  const handleDeleteService = () => {
    if (deleteTarget) deleteMutation.mutate(deleteTarget);
  };

  const changeTab = (next: StatusTab) => { setTab(next); setSearch(''); };

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Status page"
        subtitle="Configure public services and incident announcements."
        action={tab === 'services' && <Button icon="add" onClick={openCreateService}>Add service</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Services" value={stats.totalServices} icon="dns" />
        <StatCard label="Active incidents" value={stats.activeIncidents} icon="crisis_alert" color={stats.activeIncidents > 0 ? 'text-error' : undefined} />
        <StatCard label="Groups" value={stats.groups} icon="folder" />
        <StatCard label="Enabled" value={stats.enabledServices} icon="check_circle" />
      </div>

      <div>
        <Tabs
          label="Status page sections"
          idBase={tabsId}
          value={tab}
          onChange={changeTab}
          items={[
            { id: 'services', label: 'Services', icon: 'dns', badge: services.length },
            { id: 'incidents', label: 'Incidents', icon: 'crisis_alert', badge: stats.activeIncidents },
          ]}
        />
        <TabPanel idBase={tabsId} id={tab} value={tab} className="pt-4">
          <Card variant="low" className="overflow-hidden">
            <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
              <p className="text-sm text-fg-muted">
                <span className="font-semibold tabular-nums text-fg">{tab === 'services' ? filteredServices.length : filteredIncidents.length}</span> {tab}
              </p>
              <label className="relative md:w-72">
                <span className="sr-only">Search {tab}</span>
                <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
                <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={`Search ${tab}`} className={`${INPUT_CLASS} pl-10`} />
              </label>
            </div>

            {loading ? (
              <div className="p-8 text-sm text-fg-muted">Loading…</div>
            ) : tab === 'services' ? (
              filteredServices.length === 0 ? (
                <EmptyState icon="dns" title="No services configured" description="Add services to display on the public status page." action={<Button icon="add" onClick={openCreateService}>Add service</Button>} />
              ) : (
                <ul className="divide-y divide-line">
                  {filteredServices.map((s) => (
                    <li key={s.id} className="p-4 hover:bg-surface-2 transition-colors">
                      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                        <div className="flex items-center gap-4 min-w-0">
                          <StatusPill status={s.enabled ? 'up' : 'unknown'}>{s.enabled ? 'Enabled' : 'Disabled'}</StatusPill>
                          <div className="min-w-0">
                            <h3 className="font-semibold truncate">{s.name}</h3>
                            <p className="text-xs text-fg-muted mt-0.5">{s.group_name || 'Ungrouped'} · {s.aggregation?.replace('_', ' ')}</p>
                            {s.description && <p className="text-sm text-fg-muted mt-1 line-clamp-1">{s.description}</p>}
                          </div>
                        </div>
                        <div className="flex gap-2 items-center">
                          {s.show_uptime && <span className="text-xs font-medium px-2 py-0.5 rounded bg-surface-3 text-fg-muted">Uptime</span>}
                          {s.show_response_time && <span className="text-xs font-medium px-2 py-0.5 rounded bg-surface-3 text-fg-muted">Response</span>}
                          <IconButton icon="edit" label={`Edit ${s.name}`} size="sm" onClick={() => openEditService(s)} />
                          <IconButton icon="delete" label={`Delete ${s.name}`} size="sm" variant="danger" onClick={() => setDeleteTarget(s)} />
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )
            ) : filteredIncidents.length === 0 ? (
              <EmptyState icon="crisis_alert" title="No public incidents" description="Active incidents will appear here." />
            ) : (
              <ul className="divide-y divide-line">
                {filteredIncidents.map((i) => {
                  const sev = severityMeta(i.severity);
                  return (
                    <li key={i.id} className="p-4 hover:bg-surface-2 transition-colors">
                      <div className="flex items-start gap-4">
                        <div className={`w-10 h-10 rounded-md flex items-center justify-center flex-shrink-0 ${i.resolved_at ? 'bg-surface-3' : sev.chip}`}>
                          <span className={`material-symbols-outlined text-xl ${i.resolved_at ? 'text-fg-muted' : sev.text}`} aria-hidden="true">{i.resolved_at ? 'check_circle' : sev.icon}</span>
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <h3 className="font-semibold">{i.title}</h3>
                            <StatusPill status={lifecycleMeta(i.resolved_at ? 'resolved' : i.status)} />
                          </div>
                          <p className="text-xs text-fg-muted mt-0.5">
                            <span className="capitalize">{i.severity}</span> · #{i.id}
                          </p>
                          {i.message && <p className="text-sm text-fg-muted mt-2 line-clamp-2">{i.message}</p>}
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </Card>
        </TabPanel>
      </div>

      <Dialog
        open={showServiceForm}
        onClose={() => setShowServiceForm(false)}
        closeOnBackdrop={false}
        title={editingService ? 'Edit service' : 'New service'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowServiceForm(false)}>Cancel</Button>
            <Button type="submit" form="status-service-form" disabled={submitting || !serviceForm.name}>{submitting ? 'Saving…' : editingService ? 'Save changes' : 'Create service'}</Button>
          </>
        }
      >
        <form id="status-service-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleServiceSubmit(); }}>
          {[
            { key: 'name', label: 'Service name' },
            { key: 'description', label: 'Description' },
            { key: 'group_name', label: 'Group' },
          ].map(({ key, label }) => (
            <div key={key}>
              <label htmlFor={`sp_${key}`} className="text-xs font-medium text-fg-muted block mb-1.5">{label}</label>
              <input id={`sp_${key}`} value={asText(serviceForm[key])} onChange={(e) => setServiceForm({ ...serviceForm, [key]: e.target.value })} className={INPUT_CLASS} />
            </div>
          ))}
          <div>
            <label htmlFor="sp_aggregation" className="text-xs font-medium text-fg-muted block mb-1.5">Aggregation</label>
            <select id="sp_aggregation" value={asText(serviceForm.aggregation, 'any_down')} onChange={(e) => setServiceForm({ ...serviceForm, aggregation: e.target.value })} className={INPUT_CLASS}>
              <option value="any_down">Any down</option>
              <option value="all_down">All down</option>
              <option value="worst">Worst status</option>
            </select>
          </div>
          <div className="space-y-2">
            {[
              { key: 'show_uptime', label: 'Show uptime percentage' },
              { key: 'show_response_time', label: 'Show response time' },
              { key: 'enabled', label: 'Enabled' },
            ].map(({ key, label }) => (
              <label key={key} className="flex items-center gap-3 text-sm text-fg">
                <input type="checkbox" checked={!!serviceForm[key]} onChange={(e) => setServiceForm({ ...serviceForm, [key]: e.target.checked })} className="h-4 w-4 accent-primary" />
                {label}
              </label>
            ))}
          </div>
        </form>
      </Dialog>

      <ConfirmDialog open={!!deleteTarget} title="Delete service?" message={`Remove "${deleteTarget?.name}" from the status page?`} confirmLabel="Delete service" danger onConfirm={handleDeleteService} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
