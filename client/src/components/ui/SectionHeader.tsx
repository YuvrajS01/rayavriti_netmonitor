import type { ReactNode } from 'react';

interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}

export default function SectionHeader({ title, subtitle, action }: SectionHeaderProps) {
  return (
    <header className="mb-12 flex flex-col md:flex-row md:items-end justify-between gap-6">
      <div>
        <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">{title}</h1>
        {subtitle && <p className="text-on-surface-variant font-body max-w-xl">{subtitle}</p>}
      </div>
      {action && <div className="flex items-center gap-4">{action}</div>}
    </header>
  );
}
