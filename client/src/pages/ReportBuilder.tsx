import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { saveBlob } from '../api/http';
import {
  createScheduledReport, deleteScheduledReport, downloadGeneratedReport, getGeneratedReports, getScheduledReports, runScheduledReport,
  type GeneratedReport, type ScheduledReport,
} from '../api/reports';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { StatusPill } from '../components/ui/Status';
import { TabPanel, Tabs } from '../components/ui/Tabs';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { STATUS_META, type StatusMeta } from '../utils/status';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';

const EMPTY_SCHEDULED: ScheduledReport[] = [];
const EMPTY_GENERATED: GeneratedReport[] = [];

const REPORT_TYPES = ['availability', 'sla', 'mttr', 'isp', 'top_offenders', 'performance', 'health_summary'] as const;
const FORMATS = ['csv', 'html', 'pdf'] as const;
const TAB_IDS = ['scheduled', 'generated'] as const;

function runStatusMeta(status: string | null | undefined): StatusMeta {
  if (status === 'success') return { ...STATUS_META.up, label: 'Success' };
  if (status === 'failed') return { ...STATUS_META.down, label: 'Failed' };
  return { ...STATUS_META.unknown, label: status ? status.charAt(0).toUpperCase() + status.slice(1) : 'Pending' };
}

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
  const queryClient = useQueryClient();
  const [tab, setTab] = useTabParam(TAB_IDS);
  const tabsId = useTabsId();
  const [search, setSearch] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<Record<string, unknown>>({});
  const [deleteTarget, setDeleteTarget] = useState<ScheduledReport | null>(null);

  const scheduledQuery = useQuery({ queryKey: queryKeys.reports.scheduled(), queryFn: ({ signal }) => getScheduledReports({ signal }) });
  const generatedQuery = useQuery({ queryKey: queryKeys.reports.generated(), queryFn: ({ signal }) => getGeneratedReports({ signal }) });
  const loading = scheduledQuery.isPending || generatedQuery.isPending;
  const scheduled = scheduledQuery.data?.data || EMPTY_SCHEDULED;
  const generated = generatedQuery.data?.data || EMPTY_GENERATED;

  const refresh = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: queryKeys.reports.scheduled() }),
    queryClient.invalidateQueries({ queryKey: queryKeys.reports.generated() }),
  ]);

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

  const createMutation = useMutation({
    mutationFn: (body: Record<string, unknown>) => createScheduledReport(body),
    onSuccess: () => {
      addToast('Report schedule created', 'success');
      setShowForm(false);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Create failed', 'error'),
  });
  const submitting = createMutation.isPending;

  const runMutation = useMutation({
    mutationFn: (id: number) => runScheduledReport(id),
    onSuccess: () => {
      addToast('Report generation triggered', 'success');
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Run failed', 'error'),
  });

  const deleteMutation = useMutation({
    mutationFn: (report: ScheduledReport) => deleteScheduledReport(report.id),
    onSuccess: () => {
      addToast('Schedule deleted', 'success');
      setDeleteTarget(null);
      return refresh();
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
  });

  const handleSubmit = () => createMutation.mutate(form);
  const handleRunNow = (id: number) => runMutation.mutate(id);

  const handleDelete = () => {
    if (deleteTarget) deleteMutation.mutate(deleteTarget);
  };

  const handleDownload = async (report: GeneratedReport) => {
    try {
      const blob = await downloadGeneratedReport(report.id);
      saveBlob(blob, report.file_path?.split('/').pop() || `report.${report.format}`);
    } catch {
      addToast('Download failed', 'error');
    }
  };

  const inputClass = 'w-full h-control rounded-md border border-line bg-inset px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
  const shownCount = tab === 'scheduled' ? filteredScheduled.length : filteredGenerated.length;

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Report builder"
        subtitle="Schedule automated reports, review generated archives and configure recipients."
        action={<Button icon="add" onClick={openCreate}>New schedule</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Scheduled" value={stats.totalScheduled} icon="schedule" />
        <StatCard label="Active" value={stats.enabledScheduled} icon="check_circle" />
        <StatCard label="Generated" value={stats.totalGenerated} icon="description" />
        <StatCard label="Last run" value={stats.lastRun ? (formatDate(stats.lastRun).split(',')[0] ?? '') : 'None'} icon="history" />
      </div>

      <Tabs
        items={[
          { id: 'scheduled', label: 'Scheduled', icon: 'schedule', badge: scheduled.length },
          { id: 'generated', label: 'Generated', icon: 'description', badge: generated.length },
        ]}
        value={tab}
        onChange={(next) => { setTab(next); setSearch(''); }}
        label="Report archive"
        idBase={tabsId}
      />

      {TAB_IDS.map((id) => (
        <TabPanel key={id} idBase={tabsId} id={id} value={tab}>
          <Card variant="low" className="overflow-hidden">
            <div className="p-5 border-b border-line flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
              <div>
                <h2 className="font-semibold text-lg text-fg">{id === 'scheduled' ? 'Scheduled reports' : 'Generated reports'}</h2>
                <p className="text-xs text-fg-muted">{shownCount} reports</p>
              </div>
              <label className="relative md:w-64">
                <span className="sr-only">Search {id} reports</span>
                <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
                <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search reports" className={`${inputClass} pl-9`} />
              </label>
            </div>

            {loading ? (
              <div role="status" className="p-8 text-sm text-fg-muted">Loading…</div>
            ) : id === 'scheduled' ? (
              filteredScheduled.length === 0 ? (
                <EmptyState icon="schedule" title="No scheduled reports" description="Create a schedule to auto-generate reports." action={<Button icon="add" onClick={openCreate}>Create schedule</Button>} />
              ) : (
                <ul className="divide-y divide-line">
                  {filteredScheduled.map((r) => (
                    <li key={r.id} className="p-5 hover:bg-surface-2 transition-colors">
                      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                        <div className="flex items-start gap-4 min-w-0">
                          <div className="w-10 h-10 rounded-md bg-surface-2 flex items-center justify-center flex-shrink-0">
                            <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">summarize</span>
                          </div>
                          <div className="min-w-0">
                            <h3 className="font-semibold text-base text-fg truncate">{r.name}</h3>
                            <p className="text-xs text-fg-muted mt-0.5">
                              <span className="capitalize">{r.report_type.replace(/_/g, ' ')}</span> · <span className="uppercase">{r.format}</span> · <span className="font-mono">{r.schedule_cron}</span>
                            </p>
                          </div>
                        </div>
                        <div className="flex flex-wrap items-center gap-x-6 gap-y-3 flex-shrink-0">
                          <div>
                            <div className="text-xs text-fg-muted">Last run</div>
                            <div className="text-sm font-medium text-fg">{formatDate(r.last_run_at)}</div>
                          </div>
                          <div>
                            <div className="text-xs text-fg-muted mb-0.5">Status</div>
                            <StatusPill status={runStatusMeta(r.last_run_status)} />
                          </div>
                          <div className="flex items-center gap-1">
                            <Button size="sm" variant="secondary" icon="play_arrow" onClick={() => handleRunNow(r.id)}>Run now</Button>
                            <IconButton icon="delete" label={`Delete schedule ${r.name}`} variant="danger" size="sm" onClick={() => setDeleteTarget(r)} />
                          </div>
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )
            ) : filteredGenerated.length === 0 ? (
              <EmptyState icon="description" title="No generated reports" description="Reports will appear here after generation." />
            ) : (
              <ul className="divide-y divide-line">
                {filteredGenerated.map((r) => (
                  <li key={r.id} className="p-5 hover:bg-surface-2 transition-colors">
                    <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                      <div className="flex items-start gap-4 min-w-0">
                        <div className="w-10 h-10 rounded-md bg-surface-2 flex items-center justify-center flex-shrink-0">
                          <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">description</span>
                        </div>
                        <div className="min-w-0">
                          <h3 className="font-semibold text-base text-fg truncate">{r.title || r.report_type}</h3>
                          <p className="text-xs text-fg-muted mt-0.5">
                            <span className="capitalize">{r.report_type.replace(/_/g, ' ')}</span> · <span className="uppercase">{r.format}</span> · {formatSize(r.file_size_bytes)}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        <div className="text-xs text-fg-muted text-right">{formatDate(r.period_from)} – {formatDate(r.period_to)}</div>
                        <Button size="sm" variant="secondary" icon="download" onClick={() => { void handleDownload(r); }}>Download</Button>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </TabPanel>
      ))}

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        title="New report schedule"
        description="Reports are generated on the schedule and kept in the archive."
        closeOnBackdrop={false}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button onClick={handleSubmit} disabled={submitting || !form.name}>{submitting ? 'Saving…' : 'Create schedule'}</Button>
          </>
        }
      >
        <form
          className="space-y-4"
          onSubmit={(e) => { e.preventDefault(); if (form.name && !submitting) handleSubmit(); }}
        >
          <label className="block text-xs font-medium text-fg-muted">
            Name
            <input value={asText(form.name)} onChange={(e) => setForm({ ...form, name: e.target.value })} className={`${inputClass} mt-1`} placeholder="Weekly IT report" required />
          </label>
          <div className="grid grid-cols-2 gap-4">
            <label className="block text-xs font-medium text-fg-muted">
              Report type
              <select value={asText(form.report_type, 'availability')} onChange={(e) => setForm({ ...form, report_type: e.target.value })} className={`${inputClass} mt-1 capitalize`}>
                {REPORT_TYPES.map((t) => <option key={t} value={t}>{t.replace(/_/g, ' ')}</option>)}
              </select>
            </label>
            <label className="block text-xs font-medium text-fg-muted">
              Format
              <select value={asText(form.format, 'html')} onChange={(e) => setForm({ ...form, format: e.target.value })} className={`${inputClass} mt-1`}>
                {FORMATS.map((f) => <option key={f} value={f}>{f.toUpperCase()}</option>)}
              </select>
            </label>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <label className="block text-xs font-medium text-fg-muted">
              Cron schedule
              <input value={asText(form.schedule_cron)} onChange={(e) => setForm({ ...form, schedule_cron: e.target.value })} className={`${inputClass} mt-1 font-mono`} placeholder="0 9 * * *" />
            </label>
            <label className="block text-xs font-medium text-fg-muted">
              Lookback
              <select value={asText(form.lookback_period, '7d')} onChange={(e) => setForm({ ...form, lookback_period: e.target.value })} className={`${inputClass} mt-1`}>
                <option value="1d">Last 24 hours</option>
                <option value="7d">Last 7 days</option>
                <option value="30d">Last 30 days</option>
                <option value="90d">Last 90 days</option>
              </select>
            </label>
          </div>
          <label className="flex items-center gap-3 text-sm text-fg">
            <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="h-4 w-4 accent-primary" />
            Enabled
          </label>
        </form>
      </Dialog>

      <ConfirmDialog open={!!deleteTarget} title="Delete schedule?" message={`Delete "${deleteTarget?.name}"? Generated reports will be preserved.`} confirmLabel="Delete" danger onConfirm={handleDelete} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
