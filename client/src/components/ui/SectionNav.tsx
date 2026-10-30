import { NavLink } from 'react-router-dom';
import type { NavSection } from '../../navigation';

interface SectionNavProps {
  sections: NavSection[];
  /** Accessible name of the navigation landmark ("Alerts & incidents sections"). */
  label: string;
  /** `tabs` is a horizontal strip; `side` is a vertical list (Settings). */
  variant?: 'tabs' | 'side';
  className?: string;
}

/**
 * Route-level tabs: each section is its own URL, so these are links in a
 * nav landmark (with aria-current), not ARIA tabs. In-page tabs that swap
 * panels keep using <Tabs> + useTabParam.
 */
export default function SectionNav({ sections, label, variant = 'tabs', className = '' }: SectionNavProps) {
  if (variant === 'side') {
    return (
      <nav aria-label={label} className={className}>
        <ul className="flex gap-1 overflow-x-auto lg:flex-col lg:overflow-visible">
          {sections.map((s) => (
            <li key={s.id} className="shrink-0">
              <NavLink
                to={s.to}
                end={s.end}
                className={({ isActive }) =>
                  `relative flex h-9 items-center gap-2.5 rounded-md px-3 text-sm whitespace-nowrap transition-colors duration-[var(--duration-fast)] ${
                    isActive
                      ? 'bg-surface-2 font-medium text-fg lg:before:absolute lg:before:left-0 lg:before:top-2 lg:before:bottom-2 lg:before:w-0.5 lg:before:rounded-full lg:before:bg-primary'
                      : 'text-fg-muted hover:bg-surface-1 hover:text-fg'
                  }`
                }
              >
                <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{s.icon}</span>
                {s.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
    );
  }
  return (
    <nav aria-label={label} className={`mb-6 border-b border-line ${className}`}>
      <ul className="-mb-px flex gap-1 overflow-x-auto">
        {sections.map((s) => (
          <li key={s.id} className="shrink-0">
            <NavLink
              to={s.to}
              end={s.end}
              className={({ isActive }) =>
                `inline-flex h-10 items-center gap-2 border-b-2 px-3 text-sm font-medium whitespace-nowrap transition-colors duration-[var(--duration-fast)] ${
                  isActive ? 'border-primary text-fg' : 'border-transparent text-fg-muted hover:border-line-strong hover:text-fg'
                }`
              }
            >
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{s.icon}</span>
              {s.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
