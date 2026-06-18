import { memo } from 'react';

interface ResourceBarProps {
  label: string;
  value: number;
  color: string;
}

function ResourceBarInner({ label, value, color }: ResourceBarProps) {
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span>{label}</span>
        <span>{value}%</span>
      </div>
      <div className="h-2 bg-surface-container-highest rounded">
        <div className="h-2 rounded transition-[width] duration-500" style={{ width: `${Math.min(100, value)}%`, background: color }} />
      </div>
    </div>
  );
}

export const ResourceBar = memo(ResourceBarInner);
