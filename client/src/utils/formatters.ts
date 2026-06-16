export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

export function formatMbps(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
  if (value >= 1000) return `${(value / 1000).toFixed(2)} Gbps`;
  if (value >= 10) return `${value.toFixed(1)} Mbps`;
  return `${value.toFixed(2)} Mbps`;
}

export function formatLocalInput(date: Date): string {
  const d = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return d.toISOString().slice(0, 16);
}

export function formatMetricDetails(details: Record<string, unknown> | undefined, protocol: string): string {
  if (!details) return '-';
  if (protocol === 'system' || protocol === 'snmp') {
    const parts: string[] = [];
    const cpu = details.cpu as { usage?: number } | undefined;
    const memory = details.memory as { percent?: number } | undefined;
    const disk = details.disk as { percent?: number } | undefined;
    if (cpu?.usage != null) parts.push(`CPU ${cpu.usage}%`);
    if (memory?.percent != null) parts.push(`Mem ${memory.percent}%`);
    if (disk?.percent != null) parts.push(`Disk ${disk.percent}%`);
    if (parts.length > 0) return parts.join(' | ');
  }
  return JSON.stringify(details);
}
