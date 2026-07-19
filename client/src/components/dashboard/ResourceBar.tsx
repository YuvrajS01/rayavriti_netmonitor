import { memo } from 'react';

interface ResourceBarProps {
  label: string;
  value: number;
  color: string;
  warn?: boolean;
}

function ResourceBarInner({ label, value, color, warn }: ResourceBarProps) {
  const pct = Math.min(100, value);
  const fill = warn && pct > 90 ? 'var(--color-error)' : color;
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span className={warn && pct > 90 ? 'text-error font-semibold' : ''}>{label}</span>
        <span className={warn && pct > 90 ? 'text-error font-semibold' : ''}>{value}%</span>
      </div>
      <div className="h-2 bg-surface-container rounded overflow-hidden">
        <div
          className={`h-2 rounded transition-[width,box-shadow] duration-500 ${warn && pct > 90 ? 'glow-error' : ''}`}
          style={{ width: `${pct}%`, background: fill, boxShadow: warn && pct > 90 ? `0 0 10px color-mix(in oklch, ${fill} 60%, transparent)` : undefined }}
        />
      </div>
    </div>
  );
}

export const ResourceBar = memo(ResourceBarInner);
