import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: string;
  title: string;
  description?: string;
  action?: ReactNode;
  /** `compact` for empty panels inside a card. */
  size?: 'default' | 'compact';
}

export default function EmptyState({ icon = 'info', title, description, action, size = 'default' }: EmptyStateProps) {
  return (
    <div role="status" className={`flex flex-col items-center justify-center text-center ${size === 'compact' ? 'py-6' : 'py-12'}`}>
      <span className="material-symbols-outlined mb-2 text-3xl text-fg-subtle sm:text-4xl" aria-hidden="true">{icon}</span>
      <p className="text-sm font-medium text-fg">{title}</p>
      {description && <p className="mt-1 max-w-sm text-sm text-fg-muted">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
