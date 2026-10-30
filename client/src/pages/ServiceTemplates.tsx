import { useState, useEffect } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { applyServiceTemplate, getServiceTemplates, type ServiceTemplate } from '../api/serviceTemplates';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Button from '../components/ui/Button';
import Dialog from '../components/ui/Dialog';
import { StatusPill } from '../components/ui/Status';
import { severityMeta } from '../utils/status';
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
      <SectionHeader title="Service templates" subtitle="Pre-built monitoring configurations for common college infrastructure services." />

      {loading ? (
        <div className="text-sm text-fg-muted p-8">Loading templates…</div>
      ) : templates.length === 0 ? (
        <EmptyState icon="widgets" title="No templates" description="No service templates are available." />
      ) : (
        categories.map((cat) => (
          <section key={cat} className="mb-6" aria-label={cat}>
            <h2 className="text-sm font-semibold text-fg-muted mb-3">{cat}</h2>
            <ul className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {templates.filter((t) => t.category === cat).map((tmpl) => (
                <li key={tmpl.name}>
                  <button
                    type="button"
                    onClick={() => setSelected(tmpl)}
                    className="h-full w-full rounded-lg border border-line bg-surface-1 p-5 text-left transition-colors hover:border-line-strong hover:bg-surface-2"
                  >
                    <div className="flex items-start gap-3 mb-3">
                      <div className="w-10 h-10 rounded-md bg-surface-3 flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">widgets</span>
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-semibold text-base">{tmpl.name}</h3>
                        <p className="text-sm text-fg-muted mt-0.5 line-clamp-2">{tmpl.description}</p>
                      </div>
                    </div>
                    <div className="flex flex-wrap gap-4 text-xs text-fg-muted">
                      <span className="tabular-nums">{tmpl.checks?.length ?? 0} checks</span>
                      <span className="tabular-nums">{tmpl.alerts?.length ?? 0} alert rules</span>
                      <span className="font-mono uppercase">{tmpl.deviceProtocol}:{tmpl.devicePort}</span>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        ))
      )}

      <Dialog
        open={!!selected}
        onClose={() => setSelected(null)}
        title={selected?.name ?? ''}
        description={selected?.description}
        footer={
          <>
            <Button variant="secondary" onClick={() => setSelected(null)}>Cancel</Button>
            <Button type="submit" form="apply-template-form" disabled={applying || !applyForm.host}>{applying ? 'Applying…' : 'Apply template'}</Button>
          </>
        }
      >
        {selected && (
          <div className="space-y-5">
            <section>
              <h3 className="text-xs font-semibold text-fg-muted mb-2">Checks ({selected.checks?.length ?? 0})</h3>
              <ul className="space-y-2">
                {(selected.checks ?? []).map((chk) => (
                  <li key={chk.name} className="flex items-center gap-3 text-sm">
                    <span className="material-symbols-outlined text-sm text-fg-subtle" aria-hidden="true">check_circle</span>
                    <span className="flex-1">{chk.name}</span>
                    <span className="text-xs text-fg-muted font-mono">{chk.type} / {chk.interval}s</span>
                  </li>
                ))}
              </ul>
            </section>

            <section>
              <h3 className="text-xs font-semibold text-fg-muted mb-2">Alert rules ({selected.alerts?.length ?? 0})</h3>
              <ul className="space-y-2">
                {(selected.alerts ?? []).map((alert) => (
                  <li key={alert.name} className="flex items-center gap-3 text-sm">
                    <span className="flex-1">{alert.name}</span>
                    <StatusPill status={severityMeta(alert.severity)} />
                  </li>
                ))}
              </ul>
            </section>

            <form id="apply-template-form" className="border-t border-line pt-4 space-y-3" onSubmit={(e) => { e.preventDefault(); handleApply(); }}>
              <h3 className="text-xs font-semibold text-fg-muted">Apply template</h3>
              <div>
                <label htmlFor="tmpl_host" className={LABEL_CLASS}>Host / IP address<span className="text-error" aria-hidden="true"> *</span></label>
                <input id="tmpl_host" required value={applyForm.host} onChange={(e) => setApplyForm({ ...applyForm, host: e.target.value })} className={`${INPUT_CLASS} font-mono`} placeholder="e.g. 10.2.1.100" />
              </div>
              <div>
                <label htmlFor="tmpl_name" className={LABEL_CLASS}>Custom name (optional)</label>
                <input id="tmpl_name" value={applyForm.name} onChange={(e) => setApplyForm({ ...applyForm, name: e.target.value })} className={INPUT_CLASS} placeholder="e.g. CS lab ERP server" />
              </div>
              <div>
                <label htmlFor="tmpl_location" className={LABEL_CLASS}>Location ID (optional)</label>
                <input id="tmpl_location" inputMode="numeric" value={applyForm.locationId} onChange={(e) => setApplyForm({ ...applyForm, locationId: e.target.value })} className={INPUT_CLASS} placeholder="e.g. 5" />
              </div>
            </form>
          </div>
        )}
      </Dialog>
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const LABEL_CLASS = 'text-xs font-medium text-fg-muted block mb-1.5';
