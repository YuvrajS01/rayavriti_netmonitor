import { useNavigate } from 'react-router-dom';
import { logout } from '../api/client';
import { useSessionUser } from '../store/session';
import Button from '../components/ui/Button';
import SectionHeader from '../components/ui/SectionHeader';
import { DensitySwitcher, ThemeSwitcher } from '../components/ui/ThemeSwitcher';
import { useToast } from '../components/ui/useToast';

const APP_VERSION = import.meta.env.VITE_APP_VERSION || __APP_VERSION__;

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <p className="text-xs font-medium text-fg-muted">{label}</p>
      {children}
    </div>
  );
}

export default function Settings() {
  const user = useSessionUser();
  const navigate = useNavigate();
  const { addToast } = useToast();

  const handleLogout = async () => {
    const confirmed = await logout();
    if (!confirmed) addToast('Logout failed. Your session may still be active.', 'error');
    await navigate('/login');
  };

  return (
    <div>
      <SectionHeader
        title="Settings"
        subtitle="Appearance, your account and system information."
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        <section aria-labelledby="appearance-heading" className="lg:col-span-12 bg-surface-1 rounded-lg p-6 border border-line">
          <h2 id="appearance-heading" className="text-lg font-semibold mb-1 flex items-center gap-3">
            <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">contrast</span>
            Appearance
          </h2>
          <p className="text-sm text-fg-muted mb-5">
            Stored in this browser. System follows the light or dark setting of your operating system.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-2xl">
            <ThemeSwitcher />
            <DensitySwitcher />
          </div>
        </section>

        <section aria-labelledby="account-heading" className="lg:col-span-8 bg-surface-1 rounded-lg p-6 border border-line">
          <h2 id="account-heading" className="text-lg font-semibold mb-5 flex items-center gap-3">
            <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">account_circle</span>
            Account profile
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Field label="Operator name">
              <div className="w-full rounded-md border border-line bg-inset p-3 text-fg">{user?.username || 'admin'}</div>
            </Field>
            <Field label="System role">
              <div className="w-full rounded-md border border-line bg-inset p-3 text-fg-muted flex justify-between items-center">
                <span>{user?.role || 'admin'}</span>
                <span className="material-symbols-outlined text-sm" aria-label="Read-only" role="img">lock</span>
              </div>
            </Field>
          </div>
        </section>

        <section aria-labelledby="actions-heading" className="lg:col-span-4 bg-surface-1 rounded-lg p-6 border border-line">
          <h2 id="actions-heading" className="text-lg font-semibold mb-5 flex items-center gap-3">
            <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">hub</span>
            Quick actions
          </h2>
          <Button variant="danger-outline" icon="logout" onClick={handleLogout} className="w-full">
            Sign out
          </Button>
        </section>

        <section aria-labelledby="system-heading" className="lg:col-span-12 bg-surface-1 rounded-lg p-6 border border-line">
          <h2 id="system-heading" className="text-lg font-semibold mb-5 flex items-center gap-3">
            <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">info</span>
            System information
          </h2>
          <dl className="grid grid-cols-1 md:grid-cols-3 gap-6 text-sm">
            <div>
              <dt className="text-xs font-medium text-fg-muted mb-1">Version</dt>
              <dd className="font-semibold tabular-nums">NetMonitor+ v{APP_VERSION}</dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-fg-muted mb-1">Backend</dt>
              <dd className="font-semibold">Go + PostgreSQL + TimescaleDB</dd>
            </div>
            <div>
              <dt className="text-xs font-medium text-fg-muted mb-1">Frontend</dt>
              <dd className="font-semibold">React + TypeScript + Tailwind CSS</dd>
            </div>
          </dl>
        </section>
      </div>
    </div>
  );
}
