import type { ReactNode } from 'react';

interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}

export default function SectionHeader({ title, subtitle, action }: SectionHeaderProps) {
  return (
    <header className="mb-6 flex flex-col md:flex-row md:items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold text-fg mb-1">{title}</h1>
        {subtitle && <p className="text-fg-muted text-sm max-w-2xl">{subtitle}</p>}
      </div>
      {action && <div className="flex flex-wrap items-center gap-3">{action}</div>}
    </header>
  );
}
