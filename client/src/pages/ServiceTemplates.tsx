import { useState, useEffect, useCallback } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';

interface TemplateCheck {
  type: string;
  name: string;
  enabled: boolean;
  interval: number;
}

interface TemplateAlert {
  name: string;
  severity: string;
  metricField: string;
  operator: string;
  value: string;
}

interface Template {
  name: string;
  description: string;
  category: string;
  deviceProtocol: string;
  devicePort: number;
  deviceCategory: string;
  checks: TemplateCheck[];
  alerts: TemplateAlert[];
}

interface ApplyResult {
  deviceId: number;
  deviceName: string;
  sensorIds: number[];
  ruleIds: number[];
}

export default function ServiceTemplates() {
  const { addToast } = useToast();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Template | null>(null);
  const [applyForm, setApplyForm] = useState({ host: '', name: '', locationId: '' });
  const [applying, setApplying] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await v1.get('/service-templates');
      setTemplates(wrap<Template[]>(res.data).data || []);
    } catch {
      addToast('Failed to load templates', 'error');
    }
    setLoading(false);
  }, [addToast]);

  useEffect(() => { load(); }, [load]);

  const handleApply = async () => {
    if (!selected || !applyForm.host) return;
    setApplying(true);
    try {
      const body: Record<string, any> = { template: selected.name, host: applyForm.host };
      if (applyForm.name) body.name = applyForm.name;
      if (applyForm.locationId) body.locationId = parseInt(applyForm.locationId);
      const res = await v1.post('/service-templates/apply', body);
      const result = wrap<ApplyResult>(res.data).data;
      addToast(`Created device "${result?.deviceName}" with ${result?.sensorIds?.length || 0} sensors and ${result?.ruleIds?.length || 0} alert rules`, 'success');
      setSelected(null);
      setApplyForm({ host: '', name: '', locationId: '' });
    } catch (err: any) {
      addToast(err?.response?.data?.error || 'Failed to apply template', 'error');
    }
    setApplying(false);
  };

  const categories = [...new Set(templates.map((t) => t.category))].sort();

  return (
    <div>
      <SectionHeader title="Service Templates" subtitle="Pre-built monitoring configurations for common college infrastructure services" />

      {loading ? (
        <div className="text-sm text-on-surface-variant p-8">Loading templates...</div>
      ) : templates.length === 0 ? (
        <EmptyState icon="widgets" title="No templates" description="No service templates are available" />
      ) : (
        categories.map((cat) => (
          <div key={cat} className="mb-8">
            <h2 className="font-headline text-sm font-bold uppercase tracking-wide text-on-surface-variant mb-4">{cat}</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {templates.filter((t) => t.category === cat).map((tmpl) => (
                <Card key={tmpl.name} hover onClick={() => setSelected(tmpl)} className="p-5 cursor-pointer">
                  <div className="flex items-start gap-3 mb-3">
                    <div className="w-10 h-10 rounded-lg bg-primary/10 text-primary flex items-center justify-center flex-shrink-0">
                      <span className="material-symbols-outlined">widgets</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-headline font-bold text-base">{tmpl.name}</h3>
                      <p className="text-xs text-on-surface-variant mt-0.5 line-clamp-2">{tmpl.description}</p>
                    </div>
                  </div>
                  <div className="flex gap-4 text-xs text-on-surface-variant">
                    <span>{tmpl.checks.length} checks</span>
                    <span>{tmpl.alerts.length} alerts</span>
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
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0">
              <div className="flex items-center justify-between">
                <h2 className="font-headline text-lg font-bold">{selected.name}</h2>
                <button onClick={() => setSelected(null)} className="text-on-surface-variant hover:text-on-surface p-1">
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
              <p className="text-sm text-on-surface-variant mt-1">{selected.description}</p>
            </div>

            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <h3 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant mb-2">Checks ({selected.checks.length})</h3>
                <div className="space-y-2">
                  {selected.checks.map((chk) => (
                    <div key={chk.name} className="flex items-center gap-3 text-sm">
                      <span className="material-symbols-outlined text-sm text-primary">check_circle</span>
                      <span className="flex-1">{chk.name}</span>
                      <span className="text-xs text-on-surface-variant">{chk.type} / {chk.interval}s</span>
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <h3 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant mb-2">Alert Rules ({selected.alerts.length})</h3>
                <div className="space-y-2">
                  {selected.alerts.map((alert) => (
                    <div key={alert.name} className="flex items-center gap-3 text-sm">
                      <span className={`material-symbols-outlined text-sm ${alert.severity === 'critical' ? 'text-error' : 'text-warning'}`}>
                        {alert.severity === 'critical' ? 'dangerous' : 'warning'}
                      </span>
                      <span className="flex-1">{alert.name}</span>
                      <span className="text-xs text-on-surface-variant">{alert.severity}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="border-t border-outline-variant/20 pt-4">
                <h3 className="text-xs font-bold uppercase tracking-wide text-on-surface-variant mb-3">Apply Template</h3>
                <div className="space-y-3">
                  <div>
                    <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Host / IP Address *</label>
                    <input value={applyForm.host} onChange={(e) => setApplyForm({ ...applyForm, host: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. 10.2.1.100" />
                  </div>
                  <div>
                    <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Custom Name (optional)</label>
                    <input value={applyForm.name} onChange={(e) => setApplyForm({ ...applyForm, name: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. CS Lab ERP Server" />
                  </div>
                  <div>
                    <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Location ID (optional)</label>
                    <input value={applyForm.locationId} onChange={(e) => setApplyForm({ ...applyForm, locationId: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="e.g. 5" />
                  </div>
                </div>
              </div>
            </div>

            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setSelected(null)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleApply} disabled={applying || !applyForm.host} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">
                {applying ? 'Applying...' : 'Apply Template'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
