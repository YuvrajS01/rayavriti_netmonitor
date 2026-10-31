import { useId, type ReactNode } from 'react';

interface WidgetCardProps {
  title: string;
  subtitle?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}

/** The card frame for dashboard widgets that don't bring their own. */
export default function WidgetCard({ title, subtitle, action, children }: WidgetCardProps) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex h-full flex-col rounded-lg border border-line bg-surface-1 p-5">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 id={headingId} className="truncate text-sm font-semibold text-fg">{title}</h3>
          {subtitle && <p className="text-xs text-fg-muted">{subtitle}</p>}
        </div>
        {action}
      </div>
      <div className="flex min-h-0 flex-1 flex-col">{children}</div>
    </section>
  );
}

/** Placeholder body for a widget that still needs settings. */
export function NeedsSetup({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-2 py-8 text-center text-sm text-fg-muted">
      <span className="material-symbols-outlined text-[28px] text-fg-subtle" aria-hidden="true">tune</span>
      <p>{children}</p>
    </div>
  );
}

/**
 * Shown for a widget type this client doesn't know (saved by a newer
 * version, or removed): the rest of the dashboard still renders.
 */
export function UnsupportedWidget({ widget }: { widget: { type: string } }) {
  return (
    <WidgetCard title="Unsupported widget">
      <p className="py-6 text-center text-sm text-fg-muted">
        This version of NetMonitor can’t show <span className="font-mono">{widget.type}</span> widgets. You can remove it while editing.
      </p>
    </WidgetCard>
  );
}
