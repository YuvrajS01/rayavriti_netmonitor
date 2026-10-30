import type { ReactNode } from 'react';

interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}

export default function SectionHeader({ title, subtitle, action }: SectionHeaderProps) {
  return (
    <header className="mb-8 flex flex-col md:flex-row md:items-end justify-between gap-6">
      <div>
        <h1 className="font-sans text-2xl font-semibold text-fg tracking-tight mb-2">{title}</h1>
        {subtitle && <p className="text-fg-muted font-sans text-sm max-w-xl">{subtitle}</p>}
      </div>
      {action && <div className="flex items-center gap-4">{action}</div>}
    </header>
  );
}
