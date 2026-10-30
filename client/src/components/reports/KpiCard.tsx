import type { StatusMeta } from '../../utils/status';
import { StatusPill } from '../ui/Status';

interface KpiCardProps {
  icon: string;
  label: string;
  value: string | number;
  sub?: string;
  /** Optional status pill next to the label; the value itself stays in neutral ink. */
  status?: StatusMeta;
}

export default function KpiCard({ icon, label, value, sub, status }: KpiCardProps) {
  return (
    <div className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <span className="material-symbols-outlined text-[18px] text-fg-subtle" aria-hidden="true">{icon}</span>
        <span className="text-sm font-medium text-fg-muted">{label}</span>
        {status && <StatusPill status={status} className="ml-auto" />}
      </div>
      <p className="text-3xl font-semibold tabular-nums text-fg">{value}</p>
      {sub && <p className="text-xs text-fg-muted">{sub}</p>}
    </div>
  );
}
