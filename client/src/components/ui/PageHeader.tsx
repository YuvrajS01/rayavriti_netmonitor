import type { ReactNode } from 'react';
import { Link, useInRouterContext, useLocation } from 'react-router-dom';
import { breadcrumbsFor, type Crumb } from '../../navigation';
import { useSessionUser } from '../../store/session';

interface PageHeaderProps {
  title: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
  /**
   * Label of the current page when it is deeper than the nav knows about
   * (a device or incident name). Appended to the derived trail.
   */
  current?: string;
  /** Set false to hide the breadcrumb trail. */
  breadcrumbs?: boolean;
}

export function Breadcrumbs({ crumbs }: { crumbs: Crumb[] }) {
  if (crumbs.length < 2) return null;
  const last = crumbs.length - 1;
  return (
    <nav aria-label="Breadcrumb" className="mb-1">
      <ol className="flex flex-wrap items-center gap-1 text-xs text-fg-muted">
        {crumbs.map((crumb, i) => (
          <li key={`${crumb.label}-${i}`} className="inline-flex min-w-0 items-center gap-1">
            {i > 0 && <span className="material-symbols-outlined text-[14px] text-fg-subtle" aria-hidden="true">chevron_right</span>}
            {i === last ? (
              <span aria-current="page" className="truncate font-medium text-fg">{crumb.label}</span>
            ) : crumb.to ? (
              <Link to={crumb.to} className="truncate rounded-sm hover:text-fg hover:underline">{crumb.label}</Link>
            ) : (
              <span className="truncate">{crumb.label}</span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}

function RoutedBreadcrumbs({ current }: { current?: string }) {
  const { pathname } = useLocation();
  const user = useSessionUser();
  const crumbs = breadcrumbsFor(pathname, user);
  if (current) crumbs.push({ label: current });
  return <Breadcrumbs crumbs={crumbs} />;
}

/**
 * Page title block: breadcrumb trail (derived from navigation.ts), h1,
 * optional subtitle and actions.
 */
export default function PageHeader({ title, subtitle, action, current, breadcrumbs = true }: PageHeaderProps) {
  const inRouter = useInRouterContext();
  return (
    <header className="mb-6 flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div className="min-w-0">
        {breadcrumbs && inRouter && <RoutedBreadcrumbs current={current} />}
        <h1 className="mb-1 text-2xl font-semibold text-fg">{title}</h1>
        {subtitle && <p className="max-w-2xl text-sm text-fg-muted">{subtitle}</p>}
      </div>
      {action && <div className="flex flex-wrap items-center gap-3">{action}</div>}
    </header>
  );
}
