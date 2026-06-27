import { useState, useEffect, memo } from 'react';
import { NavLink, useNavigate, useLocation } from 'react-router-dom';
import { useDispatch, useSelector } from 'react-redux';
import { clearCredentials } from '../store/authSlice';
import type { RootState } from '../store';
import { logout, getAlertCounts } from '../api/client';
import { useSocket } from '../hooks/useSocket';

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

const topItems: NavItem[] = [
  { to: '/', label: 'Overview', icon: 'dashboard', end: true },
  { to: '/devices', label: 'Devices', icon: 'devices', permission: 'devices.read' },
  { to: '/alerts', label: 'Alerts', icon: 'warning', permission: 'alerts.read' },
  { to: '/incidents', label: 'Incidents', icon: 'crisis_alert', permission: 'incidents.read' },
  { to: '/ai-health', label: 'AI Health', icon: 'psychology', permission: 'insights.read' },
];

const navGroups: NavGroup[] = [
  {
    label: 'Monitor',
    items: [
      { to: '/campus', label: 'Campus', icon: 'account_tree', permission: 'locations.read' },
      { to: '/sensors', label: 'Sensors', icon: 'sensors', permission: 'devices.read' },
      { to: '/isp', label: 'ISP Links', icon: 'router', permission: 'isp.read' },
      { to: '/flows', label: 'Flows', icon: 'swap_horiz', permission: 'flows.read' },
      { to: '/capture', label: 'Capture', icon: 'network_check', permission: 'capture.read' },
    ],
  },
  {
    label: 'Manage',
    items: [
      { to: '/maintenance', label: 'Maintenance', icon: 'event_repeat', permission: 'maintenance.read' },
      { to: '/discovery', label: 'Discovery', icon: 'travel_explore', permission: 'discovery.read' },
      { to: '/service-templates', label: 'Templates', icon: 'widgets', permission: 'devices.write' },
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
    label: 'Settings',
    items: [
      { to: '/settings', label: 'General', icon: 'settings', end: true },
      { to: '/settings/locations', label: 'Locations', icon: 'apartment', permission: 'locations.write' },
      { to: '/settings/contacts', label: 'Contacts', icon: 'contacts', permission: 'contacts.write' },
      { to: '/settings/status-page', label: 'Status Page', icon: 'public', permission: 'status_page.manage' },
      { to: '/settings/users', label: 'Users & Roles', icon: 'manage_accounts', permission: 'users.manage' },
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
        `group flex items-center gap-3 py-2.5 px-5 font-label font-medium text-sm tracking-wide transition-[color,background-color,border-color] duration-200 ${
          isActive
            ? 'bg-primary/8 text-primary border-l-2 border-primary'
            : 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high border-l-2 border-transparent'
        }`
      }
    >
      <span className="material-symbols-outlined text-[18px]">{icon}</span>
      <span>{label}</span>
      {badge != null && badge > 0 && (
        <span className="ml-auto bg-error/20 text-error px-2 py-0.5 rounded-full text-[10px] font-bold min-w-[20px] text-center">
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
  const [sidebarOpen, setSidebarOpen] = useState(() => typeof window !== 'undefined' && window.innerWidth >= 1024);
  const [activeAlertCount, setActiveAlertCount] = useState(0);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({
    Monitor: false,
    Manage: false,
    Reports: false,
    Settings: false,
  });

  const hasPermission = (item: NavItem) => {
    if (!item.permission) return true;
    if (user?.role === 'super_admin' || user?.role === 'admin') return true;
    return user?.permissions?.includes(item.permission);
  };

  const visibleTopItems = topItems.filter(hasPermission);
  const visibleGroups = navGroups
    .map((g) => ({ ...g, items: g.items.filter(hasPermission) }))
    .filter((g) => g.items.length > 0);

  const isGroupActive = (group: NavGroup) =>
    group.items.some((item) => location.pathname === item.to || (!item.end && location.pathname.startsWith(item.to + '/')));

  const toggleGroup = (label: string) =>
    setExpandedGroups((prev) => ({ ...prev, [label]: !prev[label] }));

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
    onBootstrap: () => fetchAlertCount(),
  });

  useEffect(() => {
    const main = document.getElementById('main-content');
    if (main) main.focus();
  }, [location.pathname]);

  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth < 1024) setSidebarOpen(false);
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
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
      {/* Top Nav */}
      <header className="bg-background text-on-surface font-body text-sm tracking-tight w-full h-16 border-b border-surface-container-high/30 flex justify-between items-center px-6 fixed top-0 z-40">
        <div className="flex items-center gap-8">
          <button onClick={() => setSidebarOpen(!sidebarOpen)} className="material-symbols-outlined text-on-surface-variant hover:text-primary transition-colors" aria-label="Toggle sidebar">
            menu
          </button>
          <NavLink to="/" className="font-headline font-bold tracking-wide text-primary text-xl uppercase">
            Rayavriti NetMonitor+
          </NavLink>
          <div className="hidden lg:flex items-center gap-6">
            {visibleTopItems.slice(0, 3).map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `cursor-pointer transition-colors duration-300 ${
                    isActive ? 'text-primary border-b-2 border-primary' : 'text-on-surface-variant hover:text-primary'
                  }`
                }
              >
                {item.label}
              </NavLink>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-4">
          <NavLink to="/alerts" className="material-symbols-outlined cursor-pointer hover:text-primary p-2 text-on-surface-variant relative" aria-label="Alerts">
            notifications
            {activeAlertCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 bg-error text-on-error text-[8px] font-bold rounded-full w-4 h-4 flex items-center justify-center">
                {activeAlertCount > 9 ? '9+' : activeAlertCount}
              </span>
            )}
          </NavLink>
          <NavLink to="/settings" className="material-symbols-outlined cursor-pointer hover:text-primary p-2 text-on-surface-variant" aria-label="Settings">
            settings
          </NavLink>
          <div className="w-8 h-8 rounded-full bg-surface-container-highest border border-primary/20 flex items-center justify-center text-xs font-bold text-primary">
            {user?.username?.charAt(0).toUpperCase() || 'A'}
          </div>
        </div>
      </header>

      <div className="flex pt-16">
        {/* Sidebar */}
        <aside
          className={`bg-surface-container-low h-[calc(100vh-64px)] w-64 border-r border-surface-container-high/30 fixed left-0 top-16 flex flex-col z-40 transition-transform duration-300 ${
            sidebarOpen ? 'translate-x-0' : '-translate-x-full'
          }`}
          aria-hidden={!sidebarOpen}
          inert={!sidebarOpen || undefined}
        >
          <div className="px-5 py-4">
            <span className="font-headline text-primary font-bold text-base tracking-wide">
              {user?.username || 'Admin'}
            </span>
          </div>

          <nav className="flex-1 space-y-1 overflow-y-auto" aria-label="Sidebar navigation">
            {visibleTopItems.map((item) => (
              <SidebarLink key={item.to} {...item} badge={item.to === '/alerts' ? activeAlertCount : undefined} />
            ))}

            <div className="pt-2">
              {visibleGroups.map((group) => {
                const expanded = expandedGroups[group.label] ?? false;
                const active = isGroupActive(group);
                return (
                  <div key={group.label}>
                    <button
                      onClick={() => toggleGroup(group.label)}
                      className={`w-full flex items-center gap-3 py-2.5 px-5 font-label font-medium text-xs uppercase tracking-widest transition-colors ${
                        active ? 'text-primary' : 'text-on-surface-variant hover:text-on-surface'
                      }`}
                    >
                      <span className="material-symbols-outlined text-[16px]">
                        {expanded ? 'expand_more' : 'chevron_right'}
                      </span>
                      {group.label}
                      {group.label === 'Settings' && activeAlertCount > 0 && group.items.some((i) => i.to === '/alerts') && (
                        <span className="ml-auto bg-error/20 text-error px-2 py-0.5 rounded-full text-[10px] font-bold">
                          {activeAlertCount > 99 ? '99+' : activeAlertCount}
                        </span>
                      )}
                    </button>
                    <div className="animate-slide-down" data-open={expanded ? 'true' : 'false'}>
                      <div>
                        {group.items.map((item) => (
                          <SidebarLink key={item.to} {...item} />
                        ))}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </nav>

          <div className="p-5">
            <button
              onClick={handleLogout}
              className="w-full bg-surface-container-highest text-error border border-error/20 py-2.5 font-headline font-bold text-sm tracking-wide rounded-md hover:bg-error hover:text-on-error transition-[background-color,color] uppercase"
            >
              Sign Out
            </button>
          </div>
        </aside>

        {/* Main Content */}
        <main id="main-content" className={`flex-1 p-8 bg-surface min-h-[calc(100vh-64px)] transition-[margin-left] duration-300 ${sidebarOpen ? 'ml-64' : 'ml-0'}`}>
          <div key={location.pathname} className="page-enter">
            {children}
          </div>
        </main>
      </div>

      {/* Mobile Bottom Nav — only on small screens */}
      <nav className="lg:hidden fixed bottom-0 left-0 right-0 h-16 bg-background border-t border-surface-container-high/30 flex justify-around items-center px-4 z-50" aria-label="Mobile navigation">
        {[visibleTopItems[0], visibleTopItems[1], visibleTopItems.find(i => i.to === '/incidents') || visibleTopItems[2], visibleTopItems.find(i => i.to === '/alerts') || visibleTopItems[3]].filter(Boolean).map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `flex flex-col items-center gap-1 ${isActive ? 'text-primary' : 'text-on-surface-variant'}`
            }
          >
            <span className="material-symbols-outlined">{item.icon}</span>
            <span className="text-[11px] uppercase font-bold">{item.label.split(' ')[0]}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
