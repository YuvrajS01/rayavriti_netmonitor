import { useState, useEffect } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { applyServiceTemplate, getServiceTemplates, type ServiceTemplate } from '../api/serviceTemplates';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';

const EMPTY_TEMPLATES: ServiceTemplate[] = [];

export default function ServiceTemplates() {
  const { addToast } = useToast();
  const [selected, setSelected] = useState<ServiceTemplate | null>(null);
  const [applyForm, setApplyForm] = useState({ host: '', name: '', locationId: '' });

  const templatesQuery = useQuery({
    queryKey: queryKeys.serviceTemplates.list,
    queryFn: ({ signal }) => getServiceTemplates({ signal }),
  });
  const loading = templatesQuery.isPending;
  const templates = templatesQuery.data?.data || EMPTY_TEMPLATES;

  const loadFailedAt = templatesQuery.errorUpdatedAt;
  useEffect(() => {
    if (loadFailedAt) addToast('Failed to load templates', 'error');
  }, [loadFailedAt, addToast]);

  const applyMutation = useMutation({
    mutationFn: (body: Record<string, string | number | boolean>) => applyServiceTemplate(body),
    onSuccess: ({ data: result }) => {
      addToast(`Created device "${result?.deviceName}" with ${result?.sensorIds?.length || 0} sensors and ${result?.ruleIds?.length || 0} alert rules`, 'success');
      setSelected(null);
      setApplyForm({ host: '', name: '', locationId: '' });
    },
    onError: (err: unknown) => {
      addToast((err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Failed to apply template', 'error');
    },
  });
  const applying = applyMutation.isPending;

  const handleApply = () => {
    if (!selected || !applyForm.host) return;
    const body: Record<string, string | number | boolean> = { template: selected.name, host: applyForm.host };
    if (applyForm.name) body.name = applyForm.name;
    if (applyForm.locationId) body.locationId = parseInt(applyForm.locationId);
    applyMutation.mutate(body);
  };

  const categories = [...new Set(templates.map((t) => t.category))].sort();

  return (
    <div>
      <SectionHeader title="Service Templates" subtitle="Pre-built monitoring configurations for common college infrastructure services" />

      {loading ? (
        <div className="text-sm text-fg-muted p-8">Loading templates...</div>
      ) : templates.length === 0 ? (
        <EmptyState icon="widgets" title="No templates" description="No service templates are available" />
      ) : (
        categories.map((cat) => (
          <div key={cat} className="mb-6">
            <h2 className="font-sans text-sm font-semibold text-fg-muted mb-4">{cat}</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {templates.filter((t) => t.category === cat).map((tmpl) => (
                <Card key={tmpl.name} hover onClick={() => setSelected(tmpl)} className="p-5 cursor-pointer">
                  <div className="flex items-start gap-3 mb-3">
                    <div className="w-10 h-10 rounded-lg bg-primary/10 text-primary flex items-center justify-center flex-shrink-0">
                      <span className="material-symbols-outlined">widgets</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-sans font-semibold text-base">{tmpl.name}</h3>
                      <p className="text-xs text-fg-muted mt-0.5 line-clamp-2">{tmpl.description}</p>
                    </div>
                  </div>
                  <div className="flex gap-4 text-xs text-fg-muted">
                    <span>{tmpl.checks?.length ?? 0} checks</span>
                    <span>{tmpl.alerts?.length ?? 0} alerts</span>
                    <span className="uppercase">{tmpl.deviceProtocol}:{tmpl.devicePort}</span>
                  </div>
                </Card>
              ))}
            </div>
          </div>
        ))
      )}

      {selected && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setSelected(null)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0">
              <div className="flex items-center justify-between">
                <h2 className="font-sans text-lg font-semibold">{selected.name}</h2>
                <button onClick={() => setSelected(null)} className="text-fg-muted hover:text-fg p-1">
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
              <p className="text-sm text-fg-muted mt-1">{selected.description}</p>
            </div>

            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <h3 className="text-xs font-semibold text-fg-muted mb-2">Checks ({selected.checks?.length ?? 0})</h3>
                <div className="space-y-2">
                  {(selected.checks ?? []).map((chk) => (
                    <div key={chk.name} className="flex items-center gap-3 text-sm">
                      <span className="material-symbols-outlined text-sm text-primary">check_circle</span>
                      <span className="flex-1">{chk.name}</span>
                      <span className="text-xs text-fg-muted">{chk.type} / {chk.interval}s</span>
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <h3 className="text-xs font-semibold text-fg-muted mb-2">Alert Rules ({selected.alerts?.length ?? 0})</h3>
                <div className="space-y-2">
                  {(selected.alerts ?? []).map((alert) => (
                    <div key={alert.name} className="flex items-center gap-3 text-sm">
                      <span className={`material-symbols-outlined text-sm ${alert.severity === 'critical' ? 'text-error' : 'text-warning'}`}>
                        {alert.severity === 'critical' ? 'dangerous' : 'warning'}
                      </span>
                      <span className="flex-1">{alert.name}</span>
                      <span className="text-xs text-fg-muted">{alert.severity}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="border-t border-line pt-4">
                <h3 className="text-xs font-semibold text-fg-muted mb-3">Apply template</h3>
                <div className="space-y-3">
                  <div>
                    <label className="text-xs text-fg-muted block mb-1">Host / IP address *</label>
                    <input value={applyForm.host} onChange={(e) => setApplyForm({ ...applyForm, host: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. 10.2.1.100" />
                  </div>
                  <div>
                    <label className="text-xs text-fg-muted block mb-1">Custom name (optional)</label>
                    <input value={applyForm.name} onChange={(e) => setApplyForm({ ...applyForm, name: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. CS Lab ERP Server" />
                  </div>
                  <div>
                    <label className="text-xs text-fg-muted block mb-1">Location ID (optional)</label>
                    <input value={applyForm.locationId} onChange={(e) => setApplyForm({ ...applyForm, locationId: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. 5" />
                  </div>
                </div>
              </div>
            </div>

            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setSelected(null)} className="flex-1 py-3 text-xs font-sans font-semibold text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleApply} disabled={applying || !applyForm.host} className="flex-1 py-3 text-xs font-sans font-semibold text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">
                {applying ? 'Applying...' : 'Apply Template'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
