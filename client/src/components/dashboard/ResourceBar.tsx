import { memo } from 'react';
import { StatusShapeIcon } from '../ui/Status';
import { STATUS_META } from '../../utils/status';

interface ResourceBarProps {
  label: string;
  value: number;
  /** Fill colour while below the alarm threshold (a token var). */
  color: string;
  /** When true the bar is over its threshold: drawn as a down status with its shape. */
  warn?: boolean;
}

function ResourceBarInner({ label, value, color, warn }: ResourceBarProps) {
  const pct = Math.max(0, Math.min(100, value));
  const alarm = Boolean(warn);
  const down = STATUS_META.down;
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span className={`inline-flex items-center gap-1.5 ${alarm ? `${down.text} font-semibold` : 'text-fg-muted'}`}>
          {alarm && <StatusShapeIcon shape={down.shape} size={8} label="Over threshold" />}
          {label}
        </span>
        <span className={`font-mono ${alarm ? `${down.text} font-semibold` : 'text-fg'}`}>{value}%</span>
      </div>
      <div className="h-2 bg-surface-3 rounded-sm overflow-hidden">
        <div
          className="h-2 rounded-sm transition-[width] duration-[var(--duration-normal)]"
          style={{ width: `${pct}%`, background: alarm ? 'var(--color-status-down)' : color }}
        />
      </div>
    </div>
  );
}

export const ResourceBar = memo(ResourceBarInner);
