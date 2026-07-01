import { useSelector, useDispatch } from 'react-redux';
import { useNavigate } from 'react-router-dom';
import { clearCredentials } from '../store/authSlice';
import { logout } from '../api/client';
import type { RootState } from '../store';
import SectionHeader from '../components/ui/SectionHeader';
import { useToast } from '../components/ui/useToast';

const APP_VERSION = import.meta.env.VITE_APP_VERSION || '1.0.0';

export default function Settings() {
  const user = useSelector((s: RootState) => s.auth.user);
  const dispatch = useDispatch();
  const navigate = useNavigate();
  const { addToast } = useToast();

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      addToast('Logout failed. Your session may still be active.', 'error');
    }
    dispatch(clearCredentials());
    navigate('/login');
  };

  return (
    <div>
      <SectionHeader
        title="Settings"
        subtitle="Configure your account and review system information."
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Account Settings */}
        <section className="lg:col-span-8 bg-surface-container-low rounded-lg p-8 border border-outline-variant/20">
          <div className="flex items-center justify-between mb-6">
            <h2 className="font-headline text-xl font-semibold flex items-center gap-3">
              <span className="material-symbols-outlined text-primary">account_circle</span>
              Account Profile
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            <div className="space-y-2">
              <label className="font-label text-xs text-primary uppercase tracking-wide">Operator Name</label>
              <div className="w-full bg-surface-container-lowest border-0 border-b border-outline-variant/30 text-on-surface font-body p-3">
                {user?.username || 'admin'}
              </div>
            </div>
            <div className="space-y-2">
              <label className="font-label text-xs text-primary uppercase tracking-wide">System Role</label>
              <div className="w-full bg-surface-container-lowest border-b border-outline-variant/30 text-on-surface-variant font-body p-3 flex justify-between items-center">
                <span>{user?.role || 'admin'}</span>
                <span className="material-symbols-outlined text-sm">lock</span>
              </div>
            </div>
          </div>
        </section>

        {/* Quick Actions */}
        <section className="lg:col-span-4 space-y-8">
          <div className="bg-surface-container-low rounded-lg p-8 border border-outline-variant/20">
            <h2 className="font-headline text-xl font-semibold mb-6 flex items-center gap-3">
              <span className="material-symbols-outlined text-primary">hub</span>
              Quick Actions
            </h2>
            <div className="space-y-4">
              <button onClick={handleLogout} className="w-full py-3 border border-error text-error hover:bg-error hover:text-on-error font-semibold rounded-md transition-[background-color,color] text-sm uppercase tracking-wide">
                Sign Out
              </button>
            </div>
          </div>
        </section>

        {/* System Info */}
        <section className="lg:col-span-12 bg-surface-container-low rounded-lg p-8 border border-outline-variant/20">
          <h2 className="font-headline text-xl font-semibold mb-6 flex items-center gap-3">
            <span className="material-symbols-outlined text-primary">info</span>
            System Information
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 text-sm">
            <div>
              <p className="text-xs text-primary uppercase tracking-wide mb-1">Version</p>
              <p className="font-semibold">NetMonitor+ v{APP_VERSION}</p>
            </div>
            <div>
              <p className="text-xs text-primary uppercase tracking-wide mb-1">Backend</p>
              <p className="font-semibold">Go + PostgreSQL + TimescaleDB</p>
            </div>
            <div>
              <p className="text-xs text-primary uppercase tracking-wide mb-1">Frontend</p>
              <p className="font-semibold">React + TypeScript + TailwindCSS</p>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
