import { useSelector, useDispatch } from 'react-redux';
import { useNavigate } from 'react-router-dom';
import { clearCredentials } from '../store/authSlice';
import { logout } from '../api/client';
import type { RootState } from '../store';

export default function Settings() {
  const user = useSelector((s: RootState) => s.auth.user);
  const dispatch = useDispatch();
  const navigate = useNavigate();

  const handleLogout = async () => {
    try { await logout(); } catch { /* ignore */ }
    dispatch(clearCredentials());
    navigate('/login');
  };

  return (
    <div>
      <header className="mb-12">
        <h1 className="font-headline text-4xl font-black text-on-surface mb-2 tracking-tight">System Configuration</h1>
        <p className="text-on-surface-variant font-label text-sm tracking-wide">Control parameters for your monitoring node</p>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Account Settings */}
        <section className="lg:col-span-8 bg-surface-container-low rounded-xl p-8 shadow-sm">
          <div className="flex items-center justify-between mb-8">
            <h2 className="font-headline text-xl font-bold flex items-center gap-3">
              <span className="material-symbols-outlined text-primary">account_circle</span>
              Account Profile
            </h2>
            <span className="text-[10px] px-2 py-1 bg-primary/10 text-primary border border-primary/20 tracking-tighter">SECURED NODE</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            <div className="space-y-2">
              <label className="font-label text-[10px] text-primary uppercase tracking-[0.2em]">Operator Name</label>
              <div className="w-full bg-surface-container-highest border-0 border-b border-outline-variant/30 text-on-surface font-body p-3">
                {user?.username || 'admin'}
              </div>
            </div>
            <div className="space-y-2">
              <label className="font-label text-[10px] text-primary uppercase tracking-[0.2em]">System Role</label>
              <div className="w-full bg-surface-container-highest border-b border-outline-variant/30 text-on-surface font-body p-3 flex justify-between items-center opacity-70">
                <span>{user?.role || 'admin'}</span>
                <span className="material-symbols-outlined text-sm">lock</span>
              </div>
            </div>
          </div>
        </section>

        {/* Quick Actions */}
        <section className="lg:col-span-4 space-y-8">
          <div className="bg-surface-container-low rounded-xl p-8 relative overflow-hidden">
            <h2 className="font-headline text-xl font-bold mb-6 flex items-center gap-3">
              <span className="material-symbols-outlined text-primary">hub</span>
              Quick Actions
            </h2>
            <div className="space-y-4">
              <button onClick={handleLogout} className="w-full py-3 border border-error text-error hover:bg-error hover:text-on-error font-bold rounded-lg transition-all text-xs uppercase tracking-widest">
                SIGN OUT
              </button>
            </div>
          </div>
        </section>

        {/* System Info */}
        <section className="lg:col-span-12 bg-surface-container-low rounded-xl p-8">
          <h2 className="font-headline text-xl font-bold mb-8 flex items-center gap-3">
            <span className="material-symbols-outlined text-primary">info</span>
            System Information
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 text-sm">
            <div>
              <p className="text-[10px] text-primary uppercase tracking-[0.2em] mb-1">Version</p>
              <p className="font-bold">NetMonitor+ v1.0.0</p>
            </div>
            <div>
              <p className="text-[10px] text-primary uppercase tracking-[0.2em] mb-1">Backend</p>
              <p className="font-bold">Node.js + Express + SQLite</p>
            </div>
            <div>
              <p className="text-[10px] text-primary uppercase tracking-[0.2em] mb-1">Frontend</p>
              <p className="font-bold">React + TypeScript + TailwindCSS</p>
            </div>
          </div>
        </section>
      </div>

      {/* Footer */}
      <footer className="mt-20 py-8 border-t border-outline-variant/10 flex flex-col md:flex-row justify-between items-center gap-4 opacity-40 hover:opacity-100 transition-all">
        <p className="font-label text-[10px] tracking-widest text-on-surface-variant">SYSTEM VERSION 1.0.0-STABLE</p>
      </footer>
    </div>
  );
}
