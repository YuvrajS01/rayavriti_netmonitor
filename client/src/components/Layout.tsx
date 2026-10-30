import { useState, useEffect, useMemo, useRef, memo, Suspense, type FormEvent } from 'react';
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { Link, NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useSessionUser } from '../store/session';
import { logout, getAlertCounts } from '../api/client';
import { useSocketConnected } from '../hooks/useSocket';
import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '../api/queryKeys';
import IconColor from '../assets/brand/Icon-color.svg';
import ErrorBoundary from './ErrorBoundary';
import PageLoader from './ui/PageLoader';
import { ThemeSwitcher } from './ui/ThemeSwitcher';
import IconButton from './ui/IconButton';
import { StatusDot } from './ui/Status';
import { findActive, isItemActive, itemTarget, visibleGroups, type NavItem } from '../navigation';

const CommandPalette = lazyWithRetry(() => import('./CommandPalette'));

/** Desktop sidebar collapsed to a 64px icon rail; remembered per browser. */
export const NAV_RAIL_STORAGE_KEY = 'nm-nav-rail';

function readRail(): boolean {
  try {
    return window.localStorage.getItem(NAV_RAIL_STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

function writeRail(value: boolean) {
  try {
    window.localStorage.setItem(NAV_RAIL_STORAGE_KEY, value ? '1' : '0');
  } catch {
    // Storage blocked: the choice still applies for this tab.
  }
}

interface SidebarLinkProps {
  item: NavItem;
  to: string;
  active: boolean;
  rail: boolean;
  badge?: number;
  onClick?: () => void;
}

const SidebarLink = memo(function SidebarLink({ item, to, active, rail, badge, onClick }: SidebarLinkProps) {
  const count = badge != null && badge > 0 ? (badge > 99 ? '99+' : String(badge)) : null;
  return (
    <Link
      to={to}
      onClick={onClick}
      // Hubs span several URLs (and may link to a non-index section), so
      // "current" comes from navigation.ts rather than NavLink's prefix match.
      aria-current={active ? 'page' : undefined}
      title={rail ? item.label : undefined}
      className={`group relative mx-2 flex h-9 items-center rounded-md text-sm transition-colors duration-[var(--duration-fast)] ${
        rail ? 'justify-center px-0' : 'gap-3 px-3'
      } ${
        active
          ? 'bg-surface-2 font-medium text-fg before:absolute before:left-0 before:top-2 before:bottom-2 before:w-0.5 before:rounded-full before:bg-primary'
          : 'text-fg-muted hover:bg-surface-1 hover:text-fg'
      }`}
    >
      <span className="material-symbols-outlined text-[18px] text-current" aria-hidden="true">{item.icon}</span>
      <span className={rail ? 'sr-only' : 'min-w-0 flex-1 truncate'}>{item.label}</span>
      {count && (
        rail ? (
          <span className="absolute right-1 top-0.5 min-w-[18px] rounded-full bg-error px-1 text-center text-xs font-semibold leading-[18px] tabular-nums text-on-error">
            {count}
            <span className="sr-only"> active</span>
          </span>
        ) : (
          <span className="ml-auto min-w-[20px] rounded-full bg-error/12 px-2 py-0.5 text-center text-xs font-semibold tabular-nums text-error">
            {count}
            <span className="sr-only"> active</span>
          </span>
        )
      )}
    </Link>
  );
});

export default function Layout({ children }: { children?: React.ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useSessionUser();
  const connected = useSocketConnected();
  const [isDesktop, setIsDesktop] = useState(() => typeof window !== 'undefined' && window.innerWidth >= 1024);
  const [drawerOpenState, setDrawerOpen] = useState(false);
  const [rail, setRail] = useState(readRail);
  const mainRef = useRef<HTMLElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const accountRef = useRef<HTMLDivElement>(null);
  const accountButtonRef = useRef<HTMLButtonElement>(null);
  const [command, setCommand] = useState('');
  const [palette, setPalette] = useState<{ query: string } | null>(null);
  const openPalette = (query = '') => {
    setPalette({ query });
    setCommand('');
  };
  const [accountOpen, setAccountOpen] = useState(false);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});

  const groups = useMemo(() => visibleGroups(user), [user]);
  const active = findActive(location.pathname);
  const flatItems = useMemo(() => groups.flatMap((g) => g.items), [groups]);
  const mobileItems = ['overview', 'alerts', 'devices', 'topology']
    .map((id) => flatItems.find((i) => i.id === id))
    .filter((i): i is NavItem => Boolean(i));

  // The rail only exists on desktop; on narrow screens the sidebar is a drawer.
  const railed = isDesktop && rail;
  const drawerOpen = drawerOpenState && !isDesktop;
  const sidebarVisible = isDesktop || drawerOpen;

  const toggleGroup = (id: string) => setCollapsedGroups((prev) => ({ ...prev, [id]: !prev[id] }));

  const toggleNav = () => {
    if (isDesktop) {
      setRail((prev) => {
        writeRail(!prev);
        return !prev;
      });
    } else {
      setDrawerOpen((open) => !open);
    }
  };

  // The header search is the palette's entry point: typing or Enter opens
  // it with the text so far.
  const handleCommandSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    openPalette(command);
  };

  // ⌘K / Ctrl+K toggles the palette from anywhere in the app.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== 'k' || !(event.metaKey || event.ctrlKey) || event.altKey) return;
      event.preventDefault();
      setPalette((open) => (open ? null : { query: '' }));
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  // Invalidated by alert/bootstrap WebSocket events (useLiveQueryBridge).
  const alertCountsQuery = useQuery({
    queryKey: queryKeys.alerts.counts(),
    queryFn: ({ signal }) => getAlertCounts({ signal }),
  });
  const activeAlertCount = alertCountsQuery.data?.data?.active ?? 0;

  // Move focus to the content on navigation so screen readers announce the
  // new page and keyboard users don't restart from the header. <main> has
  // tabIndex={-1} so it is focusable without entering the tab order.
  const firstRender = useRef(true);
  useEffect(() => {
    if (firstRender.current) { firstRender.current = false; return; }
    mainRef.current?.focus({ preventScroll: true });
  }, [location.pathname]);

  useEffect(() => {
    const media = window.matchMedia('(min-width: 1024px)');
    const handleChange = () => {
      setIsDesktop(media.matches);
      // Leaving desktop width closes the (now overlaying) drawer.
      if (!media.matches) setDrawerOpen(false);
    };
    media.addEventListener('change', handleChange);
    return () => media.removeEventListener('change', handleChange);
  }, []);

  // Account menu: close on Escape or a press outside it.
  useEffect(() => {
    if (!accountOpen) return;
    const onPointer = (event: PointerEvent) => {
      if (!accountRef.current?.contains(event.target as Node)) setAccountOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setAccountOpen(false);
      accountButtonRef.current?.focus();
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [accountOpen]);

  // Mobile drawer: Escape closes it and returns focus to the toggle.
  useEffect(() => {
    if (!drawerOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setDrawerOpen(false);
      menuButtonRef.current?.focus();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [drawerOpen]);

  const handleLogout = async () => {
    await logout();
    await navigate('/login');
  };

  const initial = user?.username?.charAt(0).toUpperCase() || 'A';
  const navExpanded = isDesktop ? !rail : drawerOpen;
  const navToggleLabel = isDesktop ? (rail ? 'Expand navigation' : 'Collapse navigation') : drawerOpen ? 'Hide navigation' : 'Show navigation';
  const sidebarWidth = railed ? 'w-16' : 'w-[260px]';

  return (
    <div className="min-h-screen bg-canvas text-fg">
      <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-tooltip focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-on-primary">
        Skip to content
      </a>
      <header className="fixed top-0 z-header flex h-16 w-full items-center justify-between gap-3 border-b border-line bg-canvas px-4 text-sm text-fg lg:px-6">
        <div className="flex min-w-0 items-center gap-3">
          <IconButton
            ref={menuButtonRef}
            icon={isDesktop && rail ? 'menu_open' : 'menu'}
            label={navToggleLabel}
            aria-expanded={navExpanded}
            aria-controls="app-sidebar"
            onClick={toggleNav}
          />
          <div className="min-w-0">
            <div className="text-xs text-fg-muted">{active?.group.label ?? 'Monitor'}</div>
            <div className="truncate text-base font-semibold text-fg">{active?.section && (active.item.sections?.length ?? 0) > 1 ? active.section.label : active?.item.label ?? 'Overview'}</div>
          </div>
        </div>

        <form onSubmit={handleCommandSubmit} role="search" className="mx-4 hidden w-full max-w-xl md:block">
          <label className="relative block">
            <span className="sr-only">Search devices or jump to a page</span>
            <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
            <input
              type="search"
              value={command}
              onChange={(e) => openPalette(e.target.value)}
              onClick={() => openPalette(command)}
              aria-keyshortcuts="Control+K Meta+K"
              className="h-10 w-full rounded-md border border-line bg-surface-1 pl-10 pr-16 text-sm text-fg placeholder:text-fg-subtle outline-none transition-colors focus:border-primary"
              placeholder="Search devices or jump to a page"
            />
            <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded-sm border border-line-strong px-1.5 font-mono text-xs text-fg-muted" aria-hidden="true">Ctrl K</kbd>
          </label>
        </form>

        <div className="flex items-center gap-2">
          <IconButton icon="search" label="Search or jump to a page" className="md:hidden" onClick={() => openPalette()} />
          <div className="hidden items-center gap-2 rounded-md border border-line bg-surface-1 px-3 py-2 text-xs text-fg-muted sm:flex" role="status">
            <StatusDot status={connected ? 'up' : 'unknown'} size={8} label="" />
            <span>{connected ? 'Live' : 'Realtime paused'}</span>
          </div>
          <NavLink
            to="/alerts"
            end
            className={`flex h-10 items-center gap-2 rounded-md px-3 text-sm font-medium transition-colors duration-[var(--duration-fast)] ${
              activeAlertCount > 0
                ? 'bg-error/12 text-error hover:bg-error/20'
                : 'bg-surface-1 text-fg-muted hover:bg-surface-2 hover:text-fg'
            }`}
            aria-label={activeAlertCount > 0 ? `Alerts, ${activeAlertCount} active` : 'Alerts, none active'}
          >
            <span className="material-symbols-outlined text-[18px]" aria-hidden="true">notifications</span>
            <span className="hidden tabular-nums sm:inline" aria-hidden="true">{activeAlertCount > 99 ? '99+' : activeAlertCount}</span>
          </NavLink>
          <div className="relative" ref={accountRef}>
            <button
              ref={accountButtonRef}
              type="button"
              onClick={() => setAccountOpen((open) => !open)}
              className="flex h-10 items-center gap-2 rounded-md border border-line bg-surface-1 px-2 text-fg transition-colors duration-[var(--duration-fast)] hover:bg-surface-2"
              aria-label="Account menu"
              aria-expanded={accountOpen}
              aria-controls="account-menu"
            >
              <span className="flex h-6 w-6 items-center justify-center rounded-full bg-surface-3 text-xs font-semibold" aria-hidden="true">{initial}</span>
              <span className="material-symbols-outlined hidden text-[18px] text-fg-muted sm:inline" aria-hidden="true">expand_more</span>
            </button>
            {accountOpen && (
              <div id="account-menu" className="absolute right-0 top-12 z-drawer w-72 rounded-lg border border-line-strong bg-surface-1 p-2 shadow-e2">
                <div className="px-3 py-2">
                  <div className="truncate text-sm font-medium text-fg">{user?.username || 'Admin'}</div>
                  <div className="truncate text-xs text-fg-muted">{user?.role || 'admin'}</div>
                </div>
                <div className="px-3 pb-3 pt-1">
                  <ThemeSwitcher size="sm" />
                </div>
                <div className="border-t border-line pt-1">
                  <NavLink
                    to="/settings"
                    onClick={() => setAccountOpen(false)}
                    className="flex h-9 items-center gap-2 rounded-md px-3 text-sm text-fg-muted transition-colors hover:bg-surface-2 hover:text-fg"
                  >
                    <span className="material-symbols-outlined text-[18px]" aria-hidden="true">settings</span>
                    Settings
                  </NavLink>
                  <button
                    type="button"
                    onClick={handleLogout}
                    className="mt-1 flex h-9 w-full items-center gap-2 rounded-md px-3 text-left text-sm text-error transition-colors hover:bg-error/12"
                  >
                    <span className="material-symbols-outlined text-[18px]" aria-hidden="true">logout</span>
                    Sign out
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </header>

      <div className="flex pt-16">
        {drawerOpen && (
          <div className="fixed inset-0 top-16 z-drawer bg-scrim lg:hidden" aria-hidden="true" onClick={() => setDrawerOpen(false)} />
        )}
        <aside
          id="app-sidebar"
          data-rail={railed ? 'true' : 'false'}
          className={`fixed left-0 top-16 z-drawer flex h-[calc(100dvh-64px)] ${sidebarWidth} flex-col border-r border-line bg-canvas transition-[transform,width] duration-[var(--duration-normal)] ease-out ${
            sidebarVisible ? 'translate-x-0' : '-translate-x-full'
          } ${drawerOpen ? 'shadow-e3' : ''}`}
          aria-hidden={!sidebarVisible}
          inert={!sidebarVisible || undefined}
        >
          <div className={`flex h-16 shrink-0 items-center border-b border-line ${railed ? 'justify-center' : 'gap-2 px-4'}`}>
            <img src={IconColor} alt="" className="h-7 w-7" aria-hidden="true" />
            {!railed && (
              <div>
                <div className="text-base font-semibold leading-tight text-fg">rayavriti</div>
                <div className="text-xs text-fg-muted">NetMonitor</div>
              </div>
            )}
          </div>

          <nav className="flex-1 overflow-y-auto overflow-x-hidden py-3" aria-label="Main navigation">
            {groups.map((group, index) => {
              const groupActive = active?.group.id === group.id;
              const expanded = railed || groupActive || !collapsedGroups[group.id];
              const groupId = `nav-group-${group.id}`;
              const links = (
                <div className="space-y-0.5">
                  {group.items.map((item) => (
                    <SidebarLink
                      key={item.id}
                      item={item}
                      to={itemTarget(item, user)}
                      active={isItemActive(location.pathname, item)}
                      rail={railed}
                      badge={item.id === 'alerts' ? activeAlertCount : undefined}
                      onClick={() => {
                        if (!isDesktop) setDrawerOpen(false);
                      }}
                    />
                  ))}
                </div>
              );
              if (railed) {
                return (
                  <div key={group.id} role="group" aria-label={group.label} className={index > 0 ? 'mt-2 border-t border-line pt-2' : ''}>
                    {links}
                  </div>
                );
              }
              return (
                <div key={group.id} className="mb-2">
                  <button
                    type="button"
                    onClick={() => toggleGroup(group.id)}
                    aria-expanded={expanded}
                    aria-controls={groupId}
                    className={`mb-1 flex h-8 w-full items-center gap-2 px-4 text-xs font-semibold transition-colors duration-[var(--duration-fast)] ${
                      groupActive ? 'text-fg' : 'text-fg-muted hover:text-fg'
                    }`}
                  >
                    <span className="material-symbols-outlined text-[16px]" aria-hidden="true">
                      {expanded ? 'expand_more' : 'chevron_right'}
                    </span>
                    <span className="truncate">{group.label}</span>
                  </button>
                  <div id={groupId} className="animate-slide-down" data-open={expanded ? 'true' : 'false'} inert={!expanded || undefined}>
                    {links}
                  </div>
                </div>
              );
            })}
          </nav>

          {!railed && (
            <div className="mt-auto border-t border-line p-3">
              <div className="flex items-center gap-3 rounded-md bg-surface-1 p-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-surface-3 text-xs font-semibold text-fg" aria-hidden="true">
                  {initial}
                </div>
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium text-fg">{user?.username || 'Admin'}</div>
                  <div className="truncate text-xs text-fg-muted">{user?.role || 'admin'}</div>
                </div>
              </div>
            </div>
          )}
        </aside>

        <main
          id="main-content"
          ref={mainRef}
          tabIndex={-1}
          className={`min-h-[calc(100dvh-64px)] min-w-0 flex-1 bg-canvas p-4 pb-24 sm:p-6 lg:pb-6 ${railed ? 'lg:ml-16' : 'lg:ml-[260px]'}`}
        >
          <div key={location.pathname} className="page-enter">
            {/* Keyed by pathname (via the parent): a crashed or still-loading
                page stays inside the content area and navigating away resets it. */}
            <ErrorBoundary scope="route">
              <Suspense fallback={<PageLoader />}>
                {children ?? <Outlet />}
              </Suspense>
            </ErrorBoundary>
          </div>
        </main>
      </div>

      {palette && (
        <Suspense fallback={null}>
          <CommandPalette open initialQuery={palette.query} onClose={() => setPalette(null)} />
        </Suspense>
      )}

      {/* Mobile bottom nav sits on the sticky layer, under the drawer and any
          modal (dialogs use the top layer). */}
      <nav className="fixed bottom-0 left-0 right-0 z-sticky flex h-16 items-center justify-around border-t border-line bg-canvas px-4 lg:hidden" aria-label="Mobile navigation">
        {mobileItems.map((item) => {
          const isActive = isItemActive(location.pathname, item);
          return (
            <Link
              key={item.id}
              to={itemTarget(item, user)}
              aria-current={isActive ? 'page' : undefined}
              className={`flex min-w-16 flex-col items-center gap-1 rounded-md px-2 py-1 transition-colors duration-[var(--duration-fast)] ${isActive ? 'text-fg' : 'text-fg-muted'}`}
            >
              <span className="material-symbols-outlined text-[20px]" aria-hidden="true">{item.icon}</span>
              <span className="text-xs font-medium">{item.label.split(' ')[0]}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
