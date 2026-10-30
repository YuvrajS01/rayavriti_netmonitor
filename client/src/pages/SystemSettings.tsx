import { useQuery } from '@tanstack/react-query';
import { getSystemInfo } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import ErrorState from '../components/ui/ErrorState';
import { formatBytes } from '../utils/formatters';

const APP_VERSION = import.meta.env.VITE_APP_VERSION || __APP_VERSION__;

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86_400);
  const h = Math.floor((seconds % 86_400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return d > 0 ? `${d}d ${h}h` : `${h}h ${m}m`;
}

function Item({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="mb-1 text-xs font-medium text-fg-muted">{label}</dt>
      <dd className="font-semibold tabular-nums text-fg">{children}</dd>
    </div>
  );
}

/** Settings › System: version and the NetMonitor host's own resources. */
export default function SystemSettings() {
  const info = useQuery({ queryKey: queryKeys.system.info, queryFn: ({ signal }) => getSystemInfo({ signal }) });
  const host = info.data?.data;
  return (
    <div className="space-y-6">
      <SectionHeader title="System" subtitle="Version and the resources of the server NetMonitor runs on." />
      <section aria-labelledby="version-heading" className="rounded-lg border border-line bg-surface-1 p-6">
        <h2 id="version-heading" className="mb-5 text-lg font-semibold">About</h2>
        <dl className="grid grid-cols-1 gap-6 text-sm md:grid-cols-3">
          <Item label="Version">NetMonitor v{APP_VERSION}</Item>
          <Item label="Backend">Go + PostgreSQL + TimescaleDB</Item>
          <Item label="Frontend">React + TypeScript + Tailwind CSS</Item>
        </dl>
      </section>
      <section aria-labelledby="host-heading" className="rounded-lg border border-line bg-surface-1 p-6">
        <h2 id="host-heading" className="mb-5 text-lg font-semibold">Host</h2>
        {info.isError ? (
          <ErrorState message="Host information is unavailable." onRetry={() => void info.refetch()} />
        ) : !host ? (
          <p role="status" className="text-sm text-fg-muted">Loading host information…</p>
        ) : (
          <dl className="grid grid-cols-2 gap-6 text-sm md:grid-cols-4">
            <Item label="CPU">{Math.round(host.cpu.usage)}% of {host.cpu.cores} cores</Item>
            <Item label="Memory">{formatBytes(host.memory.used)} / {formatBytes(host.memory.total)}</Item>
            <Item label="Disk">{formatBytes(host.disk.used)} / {formatBytes(host.disk.total)}</Item>
            <Item label="Uptime">{formatUptime(host.uptime)}</Item>
          </dl>
        )}
      </section>
    </div>
  );
}
