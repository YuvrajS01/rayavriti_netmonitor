import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: string;
  title: string;
  description?: string;
  action?: ReactNode;
}

export default function EmptyState({ icon = 'info', title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12">
      <span className="material-symbols-outlined text-4xl mb-2">{icon}</span>
      <p className="text-xs text-on-surface-variant uppercase tracking-wide">{title}</p>
      {description && <p className="text-xs text-on-surface-variant mt-1 text-center max-w-xs">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
