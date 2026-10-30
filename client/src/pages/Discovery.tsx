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

const EMPTY_JOBS: DiscoveryJob[] = [];
const EMPTY_RESULTS: DiscoveryResult[] = [];

function formatDate(ts: string | null): string {
  if (!ts) return 'Running...';
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
    <div className="space-y-8">
      <SectionHeader
        title="Discovery Dashboard"
        subtitle="Launch subnet scans, review findings, and approve discovered devices."
        action={<Button icon="radar" onClick={() => setShowScanForm(true)}>New Scan</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Total scans" value={stats.total} icon="radar" sparklineData={jobs.map(job => job.totalIpsScanned)} />
        <StatCard label="Completed" value={stats.completed} icon="check_circle" trend="up" sparklineData={jobs.map(job => job.status === 'completed' ? 1 : 0)} />
        <StatCard label="Running" value={stats.running} icon="progress_activity" color={stats.running > 0 ? 'text-warning' : undefined} trend={stats.running ? 'up' : 'flat'} sparklineData={jobs.map(job => job.status === 'running' ? 1 : 0)} />
        <StatCard label="Devices found" value={stats.totalFound} icon="devices" sparklineData={jobs.map(job => job.devicesFound)} />
      </div>

      <Card variant="low" className="p-6 overflow-hidden"><div className="flex items-center justify-between"><div><h2 className="font-sans text-sm font-semibold">Discovery sweep</h2><p className="text-xs text-fg-muted mt-1">Live scan coverage and discovered nodes</p></div><span className={`material-symbols-outlined text-4xl text-primary ${stats.running ? 'animate-spin' : ''}`}>radar</span></div><div className="mt-6 grid grid-cols-12 gap-1.5">{Array.from({ length: 48 }, (_, index) => <i key={index} className={`h-6 rounded-sm ${index < Math.min(48, stats.totalFound) ? 'bg-success/80' : index < Math.min(48, stats.totalFound + stats.running * 8) ? 'bg-warning/60' : 'bg-inset'}`} />)}</div></Card>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-line flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">radar</span>
            </div>
            <div>
              <h2 className="font-sans font-semibold text-lg">Scan Jobs</h2>
              <p className="text-xs text-fg-muted">{filtered.length} jobs</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search scans..." className="bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="radar" title="No scans yet" description="Launch a subnet scan to discover devices on the network." action={<Button icon="radar" onClick={() => setShowScanForm(true)}>Start Scan</Button>} />
        ) : (
          <div className="divide-y divide-line">
            {filtered.map((job) => (
              <article key={job.id} className="p-5 hover:bg-surface-1/50 transition-colors cursor-pointer" onClick={() => openJob(job)}>
                <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${job.status === 'completed' ? 'bg-success/10' : job.status === 'running' ? 'bg-warning/10' : job.status === 'failed' ? 'bg-error/10' : 'bg-inset'}`}>
                      <span className={`material-symbols-outlined text-xl ${job.status === 'completed' ? 'text-success' : job.status === 'running' ? 'text-warning' : job.status === 'failed' ? 'text-error' : 'text-fg-muted'}`}>{job.status === 'completed' ? 'check_circle' : job.status === 'running' ? 'progress_activity' : 'radar'}</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-sans font-semibold text-lg font-mono">{job.subnet}</h3>
                      <p className="text-xs text-fg-muted mt-0.5 capitalize">{job.scanType.replace(/_/g, ' ')} · {job.status} · {formatDate(job.completedAt)}</p>
                      {job.errorMessage && <p className="text-sm text-error mt-1">{job.errorMessage}</p>}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:max-w-xl flex-shrink-0">
                    <div>
                      <div className="text-xs text-fg-muted">Scanned</div>
                      <div className="text-sm font-medium">{job.totalIpsScanned}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">Found</div>
                      <div className="text-sm font-semibold text-primary">{job.devicesFound}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">New</div>
                      <div className="text-sm font-medium text-success">{job.devicesNew}</div>
                    </div>
                    <div>
                      <div className="text-xs text-fg-muted">Known</div>
                      <div className="text-sm font-medium">{job.devicesKnown}</div>
                    </div>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {selectedJob && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setSelectedJob(null)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-3xl overflow-hidden max-h-[80vh] flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line flex items-center justify-between shrink-0">
              <div>
                <h2 className="font-sans text-lg font-semibold font-mono">{selectedJob.subnet}</h2>
                <p className="text-xs text-fg-muted mt-0.5 capitalize">{selectedJob.status} · {results.length} results</p>
              </div>
              <button onClick={() => setSelectedJob(null)} className="text-fg-muted hover:text-fg p-1"><span className="material-symbols-outlined">close</span></button>
            </div>
            <div className="p-6 overflow-y-auto flex-1">
              {results.length === 0 ? (
                <p className="text-sm text-fg-muted text-center py-8">No results found.</p>
              ) : (
                <div className="space-y-3">
                  {results.map((r) => (
                    <div key={r.id} className="bg-inset rounded-lg p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-sm font-semibold">{r.ipAddress}</span>
                          {r.approvedDeviceId && <span className="text-xs font-semibold px-2 py-0.5 rounded bg-success/10 text-success">Approved</span>}
                          {!r.approvedDeviceId && r.status === 'pending' && <span className="text-xs font-semibold px-2 py-0.5 rounded bg-warning/10 text-warning">Pending</span>}
                          {r.guessedCategory && <span className="text-xs font-semibold px-2 py-0.5 rounded bg-primary/10 text-primary">{r.guessedCategory.replace(/_/g, ' ')}</span>}
                          {r.guessedOs && <span className="text-xs px-2 py-0.5 rounded bg-surface-2 text-fg-muted">{r.guessedOs}</span>}
                        </div>
                        <div className="text-xs text-fg-muted mt-1 space-y-0.5">
                          {r.manufacturer && <div>Vendor: {r.manufacturer}</div>}
                          {r.hostname && <div>Host: {r.hostname}</div>}
                          {r.httpTitle && <div>HTTP: {r.httpTitle}</div>}
                          {r.sshBanner && <div>SSH: {r.sshBanner}</div>}
                          {r.tlsCertCn && <div>Cert: {r.tlsCertCn}</div>}
                          {r.snmpName && <div>SNMP: {r.snmpName}</div>}
                          {r.snmpDescription && <div>SNMP Desc: {r.snmpDescription.length > 80 ? r.snmpDescription.slice(0, 80) + '...' : r.snmpDescription}</div>}
                          {!r.manufacturer && !r.hostname && !r.httpTitle && !r.sshBanner && !r.tlsCertCn && !r.snmpName && (
                            <span>{r.macAddress || 'Unknown device'}</span>
                          )}
                        </div>
                      </div>
                      {!r.approvedDeviceId && r.status !== 'rejected' && (
                        <button onClick={(e) => { e.stopPropagation(); handleApprove(r.id); }} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1.5 rounded transition-colors whitespace-nowrap">Approve</button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {showScanForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowScanForm(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-md max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">New Subnet Scan</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-xs text-fg-muted block mb-1">Subnet (CIDR)</label>
                <input value={scanForm.subnet} onChange={(e) => setScanForm({ ...scanForm, subnet: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg font-mono placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="10.0.0.0/24" />
              </div>
              <div>
                <label className="text-xs text-fg-muted block mb-1">Scan type</label>
                <select value={scanForm.scan_type} onChange={(e) => setScanForm({ ...scanForm, scan_type: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                  <option value="ping_only">Ping Only</option>
                  <option value="ping_snmp">Ping + SNMP</option>
                  <option value="full">Full Scan</option>
                </select>
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowScanForm(false)} className="flex-1 py-3 text-xs font-sans font-semibold text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleScan} disabled={submitting || !scanForm.subnet} className="flex-1 py-3 text-xs font-sans font-semibold text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Starting...' : 'Start Scan'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
