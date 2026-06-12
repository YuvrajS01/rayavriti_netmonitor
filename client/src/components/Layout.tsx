import { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useDispatch, useSelector } from 'react-redux';
import { clearCredentials } from '../store/authSlice';
import type { RootState } from '../store';
import { logout } from '../api/client';

const navItems = [
  { to: '/', label: 'Overview', icon: 'dashboard' },
  { to: '/devices', label: 'My Devices', icon: 'devices' },
  { to: '/sensors', label: 'Monitors & Sensors', icon: 'sensors' },
  { to: '/flows', label: 'Flow Analysis', icon: 'swap_horiz' },
  { to: '/capture', label: 'Packet Capture', icon: 'network_check' },
  { to: '/ai-health', label: 'AI Health', icon: 'psychology' },
  { to: '/alerts', label: 'Alerts', icon: 'warning' },
  { to: '/reports', label: 'Reports', icon: 'analytics' },
  { to: '/settings', label: 'Settings', icon: 'settings' },
];

function SidebarLink({ to, label, icon, onClick }: { to: string; label: string; icon: string; onClick?: () => void }) {
  return (
    <NavLink
      to={to}
      end={to === '/'}
      onClick={onClick}
      className={({ isActive }) =>
        `group flex items-center gap-4 py-4 px-6 font-label font-medium text-xs uppercase tracking-[0.2em] transition-all duration-200 ${
          isActive
            ? 'bg-gradient-to-r from-primary/10 to-transparent text-primary border-l-4 border-primary shadow-[inset_10px_0_15px_-10px_rgba(217,253,58,0.3)]'
            : 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high border-l-4 border-transparent'
        }`
      }
    >
      <span className="material-symbols-outlined">{icon}</span>
      <span>{label}</span>
    </NavLink>
  );
}

export default function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const dispatch = useDispatch();
  const user = useSelector((s: RootState) => s.auth.user);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const handleLogout = async () => {
    try { await logout(); } catch { /* ignore */ }
    dispatch(clearCredentials());
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-background text-on-surface font-body">
      {/* Top Nav */}
      <header className="bg-background text-primary font-body text-sm tracking-tight w-full h-16 border-b border-surface-container-high/30 shadow-[0_0_15px_rgba(217,253,58,0.05)] flex justify-between items-center px-6 fixed top-0 z-50">
        <div className="flex items-center gap-8">
          <button onClick={() => setSidebarOpen(!sidebarOpen)} className="material-symbols-outlined text-on-surface-variant hover:text-primary transition-colors" aria-label="Toggle sidebar">
            menu
          </button>
          <NavLink to="/" className="font-headline font-black tracking-widest text-primary text-xl uppercase">
            Rayavriti NetMonitor+
          </NavLink>
          <div className="hidden lg:flex items-center gap-6">
            {navItems.slice(0, 3).map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
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
          <NavLink to="/alerts" className="material-symbols-outlined cursor-pointer hover:text-primary p-2 text-on-surface-variant" aria-label="Alerts">
            notifications
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
        >
          <div className="px-6 py-6">
            <div className="flex items-center gap-3 mb-1">
              <div className="w-2 h-2 rounded-full bg-primary animate-pulse" />
              <span className="font-headline text-primary font-bold text-sm tracking-widest uppercase">
                {user?.username || 'Admin'} Node
              </span>
            </div>
            <p className="font-label font-medium text-[10px] uppercase tracking-[0.2em] text-on-surface-variant">
              Network Ops Center
            </p>
          </div>

          <nav className="flex-1 space-y-1">
            {navItems.map((item) => (
              <SidebarLink key={item.to} {...item} />
            ))}
          </nav>

          <div className="p-6">
            <button
              onClick={handleLogout}
              className="w-full bg-surface-container-highest text-error border border-error/20 py-3 font-headline font-bold text-xs tracking-widest rounded-none hover:bg-error hover:text-on-error transition-all uppercase"
            >
              Sign Out
            </button>
          </div>
        </aside>

        {/* Main Content */}
        <main className={`flex-1 p-8 bg-surface min-h-[calc(100vh-64px)] transition-all duration-300 ${sidebarOpen ? 'ml-64' : 'ml-0'}`}>
          {children}
        </main>
      </div>

      {/* Mobile Bottom Nav — only on small screens */}
      <nav className="lg:hidden fixed bottom-0 left-0 right-0 h-16 bg-background border-t border-surface-container-high/30 flex justify-around items-center px-4 z-50 shadow-[0_-10px_20px_rgba(0,0,0,0.5)]">
        {[navItems[0], navItems[1], navItems[5], navItems[6]].map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              `flex flex-col items-center gap-1 ${isActive ? 'text-primary' : 'text-on-surface-variant'}`
            }
          >
            <span className="material-symbols-outlined">{item.icon}</span>
            <span className="text-[9px] uppercase font-bold">{item.label.split(' ')[0]}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
