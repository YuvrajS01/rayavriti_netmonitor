import { useState, useEffect, useMemo, memo, type FormEvent } from 'react';
import { NavLink, useNavigate, useLocation } from 'react-router-dom';
import { useDispatch, useSelector } from 'react-redux';
import { clearCredentials } from '../store/authSlice';
import type { RootState } from '../store';
import { logout, getAlertCounts } from '../api/client';
import { useSocket, useSocketContext } from '../hooks/useSocket';
import IconColor from '../assets/brand/Icon-color.svg';

interface NavItem {
  to: string;
  label: string;
  icon: string;
  permission?: string;
  end?: boolean;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

type VisibleNavItem = NavItem & { group: string };

const isVisibleNavItem = (item: VisibleNavItem | undefined): item is VisibleNavItem => Boolean(item);

const navGroups: NavGroup[] = [
  {
    label: 'Operations',
    items: [
      { to: '/', label: 'Overview', icon: 'dashboard', end: true },
      { to: '/alerts', label: 'Alerts', icon: 'warning', permission: 'alerts.read' },
      { to: '/incidents', label: 'Incidents', icon: 'crisis_alert', permission: 'incidents.read' },
      { to: '/ai-health', label: 'AI Health', icon: 'psychology', permission: 'insights.read' },
      { to: '/logs', label: 'Logs', icon: 'receipt_long', permission: 'system.logs' },
    ],
  },
  {
    label: 'Inventory',
    items: [
      { to: '/devices', label: 'Devices', icon: 'devices', permission: 'devices.read' },
      { to: '/campus', label: 'Campus', icon: 'account_tree', permission: 'locations.read' },
      { to: '/sensors', label: 'Sensors', icon: 'sensors', permission: 'devices.read' },
      { to: '/isp', label: 'ISP Links', icon: 'router', permission: 'isp.read' },
		{ to: '/remote', label: 'Remote Monitoring', icon: 'hub', permission: 'remote.manage' },
    ],
  },
  {
    label: 'Network Tools',
    items: [
      { to: '/discovery', label: 'Discovery', icon: 'travel_explore', permission: 'discovery.read' },
      { to: '/flows', label: 'Flows', icon: 'swap_horiz', permission: 'flows.read' },
      { to: '/capture', label: 'Packet Capture', icon: 'network_check', permission: 'capture.read' },
      { to: '/import', label: 'Bulk Import', icon: 'upload_file', permission: 'devices.write' },
    ],
  },
  {
    label: 'Reports',
    items: [
      { to: '/reports', label: 'Reports', icon: 'analytics', permission: 'reports.read', end: true },
      { to: '/reports/builder', label: 'Builder', icon: 'summarize', permission: 'reports.write' },
    ],
  },
  {
    label: 'Administration',
    items: [
      { to: '/maintenance', label: 'Maintenance', icon: 'event_repeat', permission: 'maintenance.read' },
      { to: '/service-templates', label: 'Templates', icon: 'widgets', permission: 'devices.write' },
      { to: '/settings', label: 'General', icon: 'settings', end: true },
      { to: '/settings/locations', label: 'Locations', icon: 'apartment', permission: 'locations.write' },
      { to: '/settings/contacts', label: 'Contacts', icon: 'contacts', permission: 'contacts.write' },
      { to: '/settings/status-page', label: 'Status Page', icon: 'public', permission: 'status_page.manage' },
      { to: '/settings/users', label: 'Users & Roles', icon: 'manage_accounts', permission: 'users.manage' },
      { to: '/settings/backup', label: 'Backup & Restore', icon: 'backup', permission: 'settings.write' },
    ],
  },
];

const SidebarLink = memo(function SidebarLink({ to, label, icon, badge, end, onClick }: { to: string; label: string; icon: string; badge?: number; end?: boolean; onClick?: () => void }) {
  return (
    <NavLink
      to={to}
      end={end}
      onClick={onClick}
      className={({ isActive }) =>
        `group relative mx-2 flex h-9 items-center gap-3 rounded-md px-3 font-body text-sm transition-colors duration-100 ${
          isActive
            ? 'bg-surface-container-high text-on-surface'
            : 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container-low'
        }`
      }
    >
      <span className="material-symbols-outlined text-[18px] text-current">{icon}</span>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {badge != null && badge > 0 && (
        <span className="ml-auto rounded-full bg-error/15 px-2 py-0.5 text-[10px] font-semibold text-error min-w-[20px] text-center">
          {badge > 99 ? '99+' : badge}
        </span>
      )}
    </NavLink>
  );
});

export default function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const dispatch = useDispatch();
  const user = useSelector((s: RootState) => s.auth.user);
  const { connected } = useSocketContext();
  const [sidebarOpen, setSidebarOpen] = useState(() => typeof window !== 'undefined' && window.innerWidth >= 1024);
  const [activeAlertCount, setActiveAlertCount] = useState(0);
  const [command, setCommand] = useState('');
  const [accountOpen, setAccountOpen] = useState(false);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({
    Operations: true,
    Inventory: true,
    'Network Tools': false,
    Reports: false,
    Administration: false,
  });

  const visibleGroups = useMemo(() => {
    const hasPermission = (item: NavItem) => {
      if (!item.permission) return true;
      if (user?.role === 'super_admin' || user?.role === 'admin') return true;
      return user?.permissions?.includes(item.permission);
    };

    return navGroups
      .map((g) => ({ ...g, items: g.items.filter(hasPermission) }))
      .filter((g) => g.items.length > 0);
  }, [user?.permissions, user?.role]);
  const visibleItems = useMemo(() => visibleGroups.flatMap((group) => group.items.map((item) => ({ ...item, group: group.label }))), [visibleGroups]);

  const isGroupActive = (group: NavGroup) =>
    group.items.some((item) => location.pathname === item.to || (!item.end && location.pathname.startsWith(item.to + '/')));

  const currentItem = visibleItems.find((item) =>
    location.pathname === item.to || (!item.end && location.pathname.startsWith(item.to + '/'))
  );
  const currentGroup = currentItem?.group || 'Operations';
  const mobileItems: VisibleNavItem[] = [
    visibleItems.find((i) => i.to === '/'),
    visibleItems.find((i) => i.to === '/alerts'),
    visibleItems.find((i) => i.to === '/devices'),
    visibleItems.find((i) => i.to === '/incidents'),
  ].filter(isVisibleNavItem);

  const toggleGroup = (label: string) =>
    setExpandedGroups((prev) => ({ ...prev, [label]: !prev[label] }));

  const handleCommandSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const query = command.trim();
    if (!query) return;
    const needle = query.toLowerCase();
    const match = visibleItems.find((item) =>
      item.label.toLowerCase().includes(needle) || item.to.toLowerCase().includes(needle)
    );
    if (match) {
      navigate(match.to);
    } else {
      navigate(`/devices?search=${encodeURIComponent(query)}`);
    }
    setCommand('');
  };

  const fetchAlertCount = () => {
    getAlertCounts()
      .then((res) => setActiveAlertCount(res.data?.active ?? 0))
      .catch(() => {});
  };

  useEffect(() => {
    fetchAlertCount();
  }, []);

  useSocket({
    onAlertTriggered: () => fetchAlertCount(),
    onAlertResolved: () => fetchAlertCount(),
    onBootstrap: () => fetchAlertCount(),
  });

  useEffect(() => {
    const main = document.getElementById('main-content');
    if (main) main.focus();
  }, [location.pathname]);

  useEffect(() => {
    const media = window.matchMedia('(min-width: 1024px)');
    const handleChange = () => {
      if (!media.matches) setSidebarOpen(false);
    };
    media.addEventListener('change', handleChange);
    return () => media.removeEventListener('change', handleChange);
  }, []);

  const handleLogout = async () => {
    try { await logout(); } catch { /* ignore */ }
    dispatch(clearCredentials());
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-background text-on-surface font-body">
      {/* Skip to content link for keyboard users */}
      <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:bg-primary focus:text-on-primary focus:px-4 focus:py-2 focus:rounded-lg focus:font-bold focus:text-sm">
        Skip to content
      </a>
      <header className="fixed top-0 z-40 flex h-16 w-full items-center justify-between border-b border-outline-variant/30 bg-background/95 px-4 text-sm text-on-surface backdrop-blur lg:px-6">
        <div className="flex min-w-0 items-center gap-3">
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="material-symbols-outlined flex h-9 w-9 items-center justify-center rounded-md text-on-surface-variant transition-colors duration-100 hover:bg-surface-container-low hover:text-on-surface"
            aria-label="Toggle sidebar"
          >
            menu
          </button>
          <div className="min-w-0">
            <div className="text-[11px] font-label uppercase tracking-wide text-on-surface-variant">{currentGroup}</div>
            <div className="truncate font-headline text-base font-semibold text-on-surface">{currentItem?.label || 'Overview'}</div>
          </div>
        </div>

        <form onSubmit={handleCommandSubmit} className="mx-4 hidden w-full max-w-xl md:block">
          <label className="relative block">
            <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-outline">search</span>
            <input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="h-10 w-full rounded-md border border-outline-variant/30 bg-surface-container-lowest pl-10 pr-3 text-sm text-on-surface placeholder:text-outline outline-none transition-colors focus:border-primary"
              placeholder="Search devices or jump to a page"
            />
          </label>
        </form>

        <div className="flex items-center gap-2">
          <div className="hidden items-center gap-2 rounded-md border border-outline-variant/25 bg-surface-container-low px-3 py-2 text-xs text-on-surface-variant sm:flex">
            <span className={`h-2 w-2 rounded-full ${connected ? 'bg-success' : 'bg-outline'}`} />
            <span>{connected ? 'Live' : 'Realtime paused'}</span>
          </div>
          <NavLink
            to="/alerts"
            className={`flex h-9 items-center gap-2 rounded-md px-3 text-sm transition-colors duration-100 ${
              activeAlertCount > 0
                ? 'bg-error-container text-on-error-container hover:bg-error/25'
                : 'bg-surface-container-low text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
            }`}
            aria-label="Alerts"
          >
            <span className="material-symbols-outlined text-[18px]">notifications</span>
            <span className="hidden font-medium sm:inline">{activeAlertCount > 99 ? '99+' : activeAlertCount}</span>
          </NavLink>
          <div className="relative">
            <button
              onClick={() => setAccountOpen((open) => !open)}
              className="flex h-9 items-center gap-2 rounded-md border border-outline-variant/25 bg-surface-container-low px-2 text-on-surface transition-colors duration-100 hover:bg-surface-container"
              aria-label="Account menu"
              aria-expanded={accountOpen}
            >
              <span className="flex h-6 w-6 items-center justify-center rounded-full bg-surface-container-highest text-xs font-semibold">
                {user?.username?.charAt(0).toUpperCase() || 'A'}
              </span>
              <span className="material-symbols-outlined hidden text-[18px] text-on-surface-variant sm:inline">expand_more</span>
            </button>
            {accountOpen && (
              <div className="absolute right-0 top-11 w-56 rounded-lg border border-outline-variant/25 bg-surface-container-low p-2 shadow-xl shadow-black/30">
                <div className="px-3 py-2">
                  <div className="truncate text-sm font-medium text-on-surface">{user?.username || 'Admin'}</div>
                  <div className="truncate text-xs text-on-surface-variant">{user?.role || 'admin'}</div>
                </div>
                <NavLink
                  to="/settings"
                  onClick={() => setAccountOpen(false)}
                  className="flex h-9 items-center gap-2 rounded-md px-3 text-sm text-on-surface-variant transition-colors hover:bg-surface-container hover:text-on-surface"
                >
                  <span className="material-symbols-outlined text-[18px]">settings</span>
                  Settings
                </NavLink>
                <button
                  onClick={handleLogout}
                  className="mt-1 flex h-9 w-full items-center gap-2 rounded-md px-3 text-left text-sm text-error transition-colors hover:bg-error/10"
                >
                  <span className="material-symbols-outlined text-[18px]">logout</span>
                  Sign out
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      <div className="flex pt-16">
        {/* Sidebar */}
        <aside
          className={`fixed left-0 top-16 z-40 flex h-[calc(100vh-64px)] w-[260px] transform-gpu flex-col border-r border-outline-variant/30 bg-surface-dim transition-transform duration-150 ease-out will-change-transform ${
            sidebarOpen ? 'translate-x-0' : '-translate-x-full'
          }`}
          aria-hidden={!sidebarOpen}
          inert={!sidebarOpen || undefined}
        >
          <div className="border-b border-outline-variant/20 px-4 py-4">
            <div className="flex items-center gap-2">
              <img src={IconColor} alt="" className="h-7 w-7" aria-hidden="true" />
              <div>
                <div className="font-headline text-base font-semibold leading-tight text-on-surface">rayavriti</div>
                <div className="text-[10px] font-label uppercase tracking-wide text-on-surface-variant">NetMonitor</div>
              </div>
            </div>
          </div>

          <nav className="flex-1 overflow-y-auto py-3" aria-label="Sidebar navigation">
            {visibleGroups.map((group) => {
              const active = isGroupActive(group);
              const expanded = active || (expandedGroups[group.label] ?? false);
              return (
                <div key={group.label} className="mb-2">
                  <button
                    onClick={() => toggleGroup(group.label)}
                    className={`mb-1 flex h-8 w-full items-center gap-2 px-4 font-label text-[11px] font-semibold uppercase tracking-wide transition-colors duration-100 ${
                      active ? 'text-on-surface' : 'text-on-surface-variant hover:text-on-surface'
                    }`}
                  >
                    <span className="material-symbols-outlined text-[16px]">
                      {expanded ? 'expand_more' : 'chevron_right'}
                    </span>
                    <span className="truncate">{group.label}</span>
                  </button>
                  <div className="animate-slide-down" data-open={expanded ? 'true' : 'false'}>
                    <div className="space-y-1">
                      {group.items.map((item) => (
                        <SidebarLink
                          key={item.to}
                          {...item}
                          badge={item.to === '/alerts' ? activeAlertCount : undefined}
                          onClick={() => {
                            if (window.innerWidth < 1024) setSidebarOpen(false);
                          }}
                        />
                      ))}
                    </div>
                  </div>
                </div>
              );
            })}
          </nav>

          <div className="mt-auto border-t border-outline-variant/20 p-3">
            <div className="flex items-center gap-3 rounded-lg bg-surface-container-low p-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-surface-container-highest text-xs font-semibold text-on-surface">
                {user?.username?.charAt(0).toUpperCase() || 'A'}
              </div>
              <div className="min-w-0">
                <div className="truncate text-sm font-medium text-on-surface">{user?.username || 'Admin'}</div>
                <div className="truncate text-xs text-on-surface-variant">{user?.role || 'admin'}</div>
              </div>
            </div>
          </div>
        </aside>

        <main id="main-content" className={`min-h-[calc(100vh-64px)] flex-1 bg-surface p-4 pb-20 sm:p-6 lg:pb-6 ${sidebarOpen ? 'lg:ml-[260px]' : 'ml-0'}`}>
          <div key={location.pathname} className="page-enter">
            {children}
          </div>
        </main>
      </div>

      {/* Mobile Bottom Nav — only on small screens */}
      <nav className="lg:hidden fixed bottom-0 left-0 right-0 h-16 bg-background border-t border-outline-variant/30 flex justify-around items-center px-4 z-50" aria-label="Mobile navigation">
        {mobileItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `flex flex-col items-center gap-1 transition-colors duration-100 ${isActive ? 'text-on-surface' : 'text-on-surface-variant'}`
            }
          >
            <span className="material-symbols-outlined text-[20px]">{item.icon}</span>
            <span className="text-[10px] font-medium">{item.label.split(' ')[0]}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
