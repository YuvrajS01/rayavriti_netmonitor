import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { approveDiscoveryResult, getDiscoveryJobs, getDiscoveryResults, startDiscoveryScan, type DiscoveryJob, type DiscoveryResult } from '../api/discovery';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import { useToast } from '../components/ui/useToast';
import Dialog from '../components/ui/Dialog';
import { StatusPill } from '../components/ui/Status';
import { STATUS_META, type StatusMeta } from '../utils/status';

const EMPTY_JOBS: DiscoveryJob[] = [];
const EMPTY_RESULTS: DiscoveryResult[] = [];

function formatDate(ts: string | null): string {
  if (!ts) return 'In progress';
  return new Date(ts).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short' });
}

export default function Discovery() {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [selectedJob, setSelectedJob] = useState<DiscoveryJob | null>(null);
  const [showScanForm, setShowScanForm] = useState(false);
  const [scanForm, setScanForm] = useState({ subnet: '', scan_type: 'ping_only' });

  const jobsQuery = useQuery({
    queryKey: queryKeys.discovery.jobs(),
    queryFn: ({ signal }) => getDiscoveryJobs({ signal }),
  });
  const resultsQuery = useQuery({
    queryKey: queryKeys.discovery.results(selectedJob?.id ?? 0),
    queryFn: ({ signal }) => getDiscoveryResults(selectedJob?.id ?? 0, { signal }),
    enabled: selectedJob != null,
  });
  const loading = jobsQuery.isPending;
  const jobs = jobsQuery.data?.data || EMPTY_JOBS;
  const results = resultsQuery.data?.data || EMPTY_RESULTS;

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return jobs.filter((j) => !needle || j.subnet.toLowerCase().includes(needle) || j.status.toLowerCase().includes(needle));
  }, [jobs, search]);

  const stats = useMemo(() => ({
    total: jobs.length,
    completed: jobs.filter((j) => j.status === 'completed').length,
    running: jobs.filter((j) => j.status === 'running').length,
    totalFound: jobs.reduce((s, j) => s + (j.devicesFound || 0), 0),
  }), [jobs]);

  const scanMutation = useMutation({
    mutationFn: (body: typeof scanForm) => startDiscoveryScan(body),
    onSuccess: () => {
      addToast('Scan initiated', 'success');
      setShowScanForm(false);
      setScanForm({ subnet: '', scan_type: 'ping_only' });
      return queryClient.invalidateQueries({ queryKey: queryKeys.discovery.jobs() });
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Scan failed', 'error'),
  });
  const submitting = scanMutation.isPending;

  const approveMutation = useMutation({
    mutationFn: (resultId: number) => approveDiscoveryResult(resultId),
    onSuccess: () => {
      addToast('Device approved', 'success');
      return queryClient.invalidateQueries({ queryKey: queryKeys.discovery.all });
    },
    onError: (err) => addToast(err instanceof Error ? err.message : 'Approve failed', 'error'),
  });

  const handleScan = () => scanMutation.mutate(scanForm);
  const openJob = (job: DiscoveryJob) => setSelectedJob(job);
  const handleApprove = (resultId: number) => approveMutation.mutate(resultId);

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Discovery"
        subtitle="Launch subnet scans, review findings, and approve discovered devices."
        action={<Button icon="radar" onClick={() => setShowScanForm(true)}>New scan</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total scans" value={stats.total} icon="radar" />
        <StatCard label="Completed" value={stats.completed} icon="check_circle" />
        <StatCard label="Running" value={stats.running} icon="progress_activity" />
        <StatCard label="Devices found" value={stats.totalFound} icon="devices" />
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
          <h2 className="text-base font-semibold">Scan jobs <span className="text-sm font-normal text-fg-muted tabular-nums">({filtered.length})</span></h2>
          <label className="relative md:w-64">
            <span className="sr-only">Search scans</span>
            <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
            <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search scans" className={`${INPUT_CLASS} pl-10`} />
          </label>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading…</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="radar" title="No scans yet" description="Launch a subnet scan to discover devices on the network." action={<Button icon="radar" onClick={() => setShowScanForm(true)}>Start scan</Button>} />
        ) : (
          <ul className="divide-y divide-line">
            {filtered.map((job) => {
              const meta = jobMeta(job.status);
              return (
                <li key={job.id}>
                  <button
                    type="button"
                    onClick={() => openJob(job)}
                    className="w-full p-4 text-left hover:bg-surface-2 transition-colors"
                    aria-label={`Open results for ${job.subnet}`}
                  >
                    <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="font-semibold font-mono">{job.subnet}</h3>
                          <StatusPill status={meta} />
                        </div>
                        <p className="text-xs text-fg-muted mt-1">
                          <span className="capitalize">{job.scanType.replace(/_/g, ' ')}</span> · {formatDate(job.completedAt)}
                        </p>
                        {job.errorMessage && <p className="text-sm text-error mt-1">{job.errorMessage}</p>}
                      </div>
                      <dl className="grid grid-cols-4 gap-4 lg:w-96 flex-shrink-0">
                        {[
                          ['Scanned', job.totalIpsScanned],
                          ['Found', job.devicesFound],
                          ['New', job.devicesNew],
                          ['Known', job.devicesKnown],
                        ].map(([label, value]) => (
                          <div key={label}>
                            <dt className="text-xs text-fg-muted">{label}</dt>
                            <dd className="text-sm font-semibold tabular-nums">{value}</dd>
                          </div>
                        ))}
                      </dl>
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </Card>

      <Dialog
        open={!!selectedJob}
        onClose={() => setSelectedJob(null)}
        size="xl"
        title={<span className="font-mono">{selectedJob?.subnet}</span>}
        description={selectedJob ? `${jobMeta(selectedJob.status).label} · ${results.length} results` : undefined}
      >
        {results.length === 0 ? (
          <p className="text-sm text-fg-muted text-center py-8">No results found.</p>
        ) : (
          <ul className="space-y-3">
            {results.map((r) => (
              <li key={r.id} className="bg-inset border border-line rounded-md p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-mono text-sm font-semibold">{r.ipAddress}</span>
                    {r.approvedDeviceId && <StatusPill status="up">Approved</StatusPill>}
                    {!r.approvedDeviceId && r.status === 'pending' && <StatusPill status="unknown">Pending</StatusPill>}
                    {r.guessedCategory && <span className="text-xs font-medium px-2 py-0.5 rounded bg-surface-3 text-fg-muted capitalize">{r.guessedCategory.replace(/_/g, ' ')}</span>}
                    {r.guessedOs && <span className="text-xs px-2 py-0.5 rounded bg-surface-3 text-fg-muted">{r.guessedOs}</span>}
                  </div>
                  <div className="text-xs text-fg-muted mt-1 space-y-0.5">
                    {r.manufacturer && <div>Vendor: {r.manufacturer}</div>}
                    {r.hostname && <div>Host: {r.hostname}</div>}
                    {r.httpTitle && <div>HTTP: {r.httpTitle}</div>}
                    {r.sshBanner && <div>SSH: {r.sshBanner}</div>}
                    {r.tlsCertCn && <div>Cert: {r.tlsCertCn}</div>}
                    {r.snmpName && <div>SNMP: {r.snmpName}</div>}
                    {r.snmpDescription && <div>SNMP description: {r.snmpDescription.length > 80 ? r.snmpDescription.slice(0, 80) + '…' : r.snmpDescription}</div>}
                    {!r.manufacturer && !r.hostname && !r.httpTitle && !r.sshBanner && !r.tlsCertCn && !r.snmpName && (
                      <span className="font-mono">{r.macAddress || 'Unknown device'}</span>
                    )}
                  </div>
                </div>
                {!r.approvedDeviceId && r.status !== 'rejected' && (
                  <Button variant="primary-outline" size="sm" icon="add_task" onClick={() => handleApprove(r.id)} aria-label={`Approve ${r.ipAddress}`}>Approve</Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Dialog>

      <Dialog
        open={showScanForm}
        onClose={() => setShowScanForm(false)}
        size="sm"
        title="New subnet scan"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowScanForm(false)}>Cancel</Button>
            <Button type="submit" form="scan-form" icon="radar" disabled={submitting || !scanForm.subnet}>{submitting ? 'Starting…' : 'Start scan'}</Button>
          </>
        }
      >
        <form id="scan-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleScan(); }}>
          <div>
            <label htmlFor="scan-subnet" className="text-xs font-medium text-fg-muted block mb-1.5">Subnet (CIDR)</label>
            <input id="scan-subnet" required value={scanForm.subnet} onChange={(e) => setScanForm({ ...scanForm, subnet: e.target.value })} className={`${INPUT_CLASS} font-mono`} placeholder="10.0.0.0/24" />
          </div>
          <div>
            <label htmlFor="scan-type" className="text-xs font-medium text-fg-muted block mb-1.5">Scan type</label>
            <select id="scan-type" value={scanForm.scan_type} onChange={(e) => setScanForm({ ...scanForm, scan_type: e.target.value })} className={INPUT_CLASS}>
              <option value="ping_only">Ping only</option>
              <option value="ping_snmp">Ping + SNMP</option>
              <option value="full">Full scan</option>
            </select>
          </div>
        </form>
      </Dialog>
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';

/** Scan job state through the status system (running = in progress, not a fault). */
function jobMeta(status: string): StatusMeta {
  if (status === 'completed') return { ...STATUS_META.up, label: 'Completed' };
  if (status === 'running') return { ...STATUS_META.info, label: 'Running' };
  if (status === 'failed') return { ...STATUS_META.down, label: 'Failed' };
  return { ...STATUS_META.unknown, label: status ? status.charAt(0).toUpperCase() + status.slice(1) : 'Unknown' };
}
