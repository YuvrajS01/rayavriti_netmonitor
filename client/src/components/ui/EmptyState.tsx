import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: string;
  title: string;
  description?: string;
  action?: ReactNode;
}

export default function EmptyState({ icon = 'info', title, description, action }: EmptyStateProps) {
  return (
    <div role="region" aria-label={title} className="flex flex-col items-center justify-center py-12">
      <span className="material-symbols-outlined text-3xl sm:text-4xl mb-2 text-fg-muted">{icon}</span>
      <p className="text-xs text-fg-muted uppercase tracking-wide font-sans font-medium">{title}</p>
      {description && <p className="text-xs text-fg-muted mt-1 text-center max-w-xs">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
