import { useState, useEffect, useMemo } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface ScheduledReport {
  id: number;
  name: string;
  report_type: string;
  format: string;
  schedule_cron: string;
  timezone: string;
  scope_type: string;
  recipients: string[];
  lookback_period: string;
  last_run_at: string | null;
  last_run_status: string | null;
  enabled: boolean;
}

interface GeneratedReport {
  id: number;
  scheduled_report_id: number;
  report_type: string;
  title: string;
  format: string;
  file_path: string;
  file_size_bytes: number;
  period_from: string;
  period_to: string;
  generated_by: string;
}

const REPORT_TYPES = ['availability', 'sla', 'mttr', 'isp', 'top_offenders', 'performance', 'health_summary'] as const;
const FORMATS = ['csv', 'html', 'pdf'] as const;

function formatDate(ts: string | null): string {
  if (!ts) return 'Never';
  return new Date(ts).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short' });
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

export default function ReportBuilder() {
  const { addToast } = useToast();
  const [scheduled, setScheduled] = useState<ScheduledReport[]>([]);
  const [generated, setGenerated] = useState<GeneratedReport[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<'scheduled' | 'generated'>('scheduled');
  const [search, setSearch] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ScheduledReport | null>(null);

  const load = async () => {
    setLoading(true);
    const [schedRes, genRes] = await Promise.all([
      v1.get('/reports/scheduled'),
      v1.get('/reports/generated'),
    ]);
    setScheduled(wrap<ScheduledReport[]>(schedRes.data).data || []);
    setGenerated(wrap<GeneratedReport[]>(genRes.data).data || []);
    setLoading(false);
  };

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const [schedRes, genRes] = await Promise.all([
          v1.get('/reports/scheduled'),
          v1.get('/reports/generated'),
        ]);
        if (active) setScheduled(wrap<ScheduledReport[]>(schedRes.data).data || []);
        if (active) setGenerated(wrap<GeneratedReport[]>(genRes.data).data || []);
      } catch {
        if (active) {
          setScheduled([]);
          setGenerated([]);
        }
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, []);

  const filteredScheduled = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return scheduled.filter((r) => !needle || r.name.toLowerCase().includes(needle) || r.report_type.toLowerCase().includes(needle));
  }, [scheduled, search]);

  const filteredGenerated = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return generated.filter((r) => !needle || r.title?.toLowerCase().includes(needle) || r.report_type.toLowerCase().includes(needle));
  }, [generated, search]);

  const stats = useMemo(() => ({
    totalScheduled: scheduled.length,
    enabledScheduled: scheduled.filter((r) => r.enabled).length,
    totalGenerated: generated.length,
    lastRun: scheduled.reduce((latest, r) => {
      if (!r.last_run_at) return latest;
      return !latest || r.last_run_at > latest ? r.last_run_at : latest;
    }, null as string | null),
  }), [scheduled, generated]);

  const openCreate = () => {
    setForm({ name: '', report_type: 'availability', format: 'html', schedule_cron: '0 9 * * *', lookback_period: '7d', scope_type: 'global', recipients: [], enabled: true });
    setShowForm(true);
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      await v1.post('/reports/scheduled', form);
      addToast('Report schedule created', 'success');
      setShowForm(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Create failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleRunNow = async (id: number) => {
    try {
      await v1.post(`/reports/scheduled/${id}/run`);
      addToast('Report generation triggered', 'success');
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Run failed', 'error');
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await v1.delete(`/reports/scheduled/${deleteTarget.id}`);
      addToast('Schedule deleted', 'success');
      setDeleteTarget(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  const handleDownload = async (report: GeneratedReport) => {
    try {
      const res = await v1.get(`/reports/generated/${report.id}/download`, { responseType: 'blob' });
      const url = URL.createObjectURL(res.data as Blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = report.file_path?.split('/').pop() || `report.${report.format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      addToast('Download failed', 'error');
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="Report Builder"
        subtitle="Schedule automated reports, review generated archives, and configure recipients."
        action={<Button icon="add" onClick={openCreate}>New Schedule</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Scheduled" value={stats.totalScheduled} icon="schedule" />
        <StatCard label="Active" value={stats.enabledScheduled} icon="check_circle" />
        <StatCard label="Generated" value={stats.totalGenerated} icon="description" />
        <StatCard label="Last Run" value={stats.lastRun ? formatDate(stats.lastRun).split(',')[0] : 'None'} icon="history" />
      </div>

      <div className="flex gap-1 bg-surface-container-low rounded-lg p-1 w-fit">
        {(['scheduled', 'generated'] as const).map((t) => (
          <button key={t} onClick={() => { setTab(t); setSearch(''); }} className={`px-4 py-2 text-xs font-headline font-bold uppercase tracking-wide rounded-md transition-colors ${tab === t ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:text-on-surface'}`}>
            {t === 'scheduled' ? 'Scheduled' : 'Generated'}
          </button>
        ))}
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">{tab === 'scheduled' ? 'schedule' : 'description'}</span>
            </div>
            <div>
              <h2 className="font-headline font-bold text-lg">{tab === 'scheduled' ? 'Scheduled Reports' : 'Generated Reports'}</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{tab === 'scheduled' ? filteredScheduled.length : filteredGenerated.length} reports</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : tab === 'scheduled' ? (
          filteredScheduled.length === 0 ? (
            <EmptyState icon="schedule" title="No scheduled reports" description="Create a schedule to auto-generate reports." action={<Button icon="add" onClick={openCreate}>Create Schedule</Button>} />
          ) : (
            <div className="divide-y divide-outline-variant/20">
              {filteredScheduled.map((r) => (
                <article key={r.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-lg bg-surface-container-highest flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">summarize</span>
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-headline font-bold text-lg truncate">{r.name}</h3>
                        <p className="text-xs text-on-surface-variant mt-0.5 capitalize">{r.report_type.replace(/_/g, ' ')} · {r.format.toUpperCase()} · {r.schedule_cron}</p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 lg:max-w-lg flex-shrink-0">
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Last Run</div>
                        <div className="text-sm font-medium">{formatDate(r.last_run_at)}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Status</div>
                        <div className={`text-sm font-medium ${r.last_run_status === 'success' ? 'text-success' : r.last_run_status === 'failed' ? 'text-error' : 'text-outline'}`}>{r.last_run_status || 'Pending'}</div>
                      </div>
                      <div className="flex items-end gap-2">
                        <button onClick={() => handleRunNow(r.id)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Run Now</button>
                        <button onClick={() => setDeleteTarget(r)} className="text-xs font-bold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                      </div>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )
        ) : filteredGenerated.length === 0 ? (
          <EmptyState icon="description" title="No generated reports" description="Reports will appear here after generation." />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filteredGenerated.map((r) => (
              <article key={r.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className="w-10 h-10 rounded-lg bg-surface-container-highest flex items-center justify-center flex-shrink-0">
                      <span className="material-symbols-outlined text-primary text-xl">description</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-headline font-bold text-lg truncate">{r.title || r.report_type}</h3>
                      <p className="text-xs text-on-surface-variant mt-0.5 capitalize">{r.report_type.replace(/_/g, ' ')} · {r.format.toUpperCase()} · {formatSize(r.file_size_bytes)}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-xs text-on-surface-variant text-right">
                      <div>{formatDate(r.period_from)} – {formatDate(r.period_to)}</div>
                    </div>
                    <button onClick={() => handleDownload(r)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Download</button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0"><h2 className="font-headline text-lg font-bold">New Report Schedule</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Name</label>
                <input value={String(form.name ?? '')} onChange={(e) => setForm({ ...form, name: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="Weekly IT Report" />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Report Type</label>
                  <select value={String(form.report_type ?? 'availability')} onChange={(e) => setForm({ ...form, report_type: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                    {REPORT_TYPES.map((t) => <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>)}
                  </select>
                </div>
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Format</label>
                  <select value={String(form.format ?? 'html')} onChange={(e) => setForm({ ...form, format: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                    {FORMATS.map((f) => <option key={f} value={f}>{f.toUpperCase()}</option>)}
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Cron Schedule</label>
                  <input value={String(form.schedule_cron ?? '')} onChange={(e) => setForm({ ...form, schedule_cron: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="0 9 * * *" />
                </div>
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Lookback</label>
                  <select value={String(form.lookback_period ?? '7d')} onChange={(e) => setForm({ ...form, lookback_period: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                    <option value="1d">Last 24 hours</option>
                    <option value="7d">Last 7 days</option>
                    <option value="30d">Last 30 days</option>
                    <option value="90d">Last 90 days</option>
                  </select>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="accent-primary" id="rpt_enabled" />
                <label htmlFor="rpt_enabled" className="text-sm text-on-surface">Enabled</label>
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowForm(false)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleSubmit} disabled={submitting || !form.name} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : 'Create'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete Schedule" message={`Delete "${deleteTarget?.name}"? Generated reports will be preserved.`} confirmLabel="Delete" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
