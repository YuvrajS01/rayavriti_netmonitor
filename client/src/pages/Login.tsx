import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { login } from '../api/client';
import LockupColor from '../assets/brand/Lockup-color.svg';
import IconColor from '../assets/brand/Icon-color.svg';
import Button from '../components/ui/Button';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username.trim(), password);
      await navigate('/');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Login failed';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-canvas text-fg min-h-screen flex">
      {/* Brand panel */}
      <div
        className="hidden lg:flex lg:w-1/2 bg-surface-1 border-r border-line items-center justify-center"
        style={{
          backgroundImage: 'radial-gradient(circle, var(--color-line-strong) 1px, transparent 1px)',
          backgroundSize: '24px 24px',
        }}
      >
        <img src={LockupColor} alt="Rayavriti" className="max-w-xs" />
      </div>

      <main className="w-full lg:w-1/2 flex items-center justify-center px-6 py-12">
        <div className="w-full max-w-md">
          <img src={IconColor} alt="Rayavriti" className="w-10 h-10 mb-6 lg:hidden" />

          <div className="login-card rounded-lg border border-line bg-surface-1 p-8 w-full">
            <h1 className="text-2xl font-semibold text-fg mb-1">Sign in</h1>
            <p className="text-sm text-fg-muted mb-8">NetMonitor operator console</p>

            <form className="space-y-5" onSubmit={handleSubmit} noValidate>
              <div className="space-y-1.5">
                <label htmlFor="login-username" className="text-xs font-medium text-fg-muted">Username</label>
                <div className="relative flex items-center rounded-md border border-line bg-inset px-3 focus-within:border-primary">
                  <span className="material-symbols-outlined text-fg-subtle text-lg mr-2" aria-hidden="true">person</span>
                  <input
                    id="login-username"
                    className="h-11 w-full bg-transparent text-sm text-fg placeholder:text-fg-subtle outline-none"
                    type="text"
                    autoComplete="username"
                    autoFocus
                    required
                    aria-invalid={error ? true : undefined}
                    aria-describedby={error ? 'login-error' : undefined}
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                  />
                </div>
              </div>

              <div className="space-y-1.5">
                <label htmlFor="login-password" className="text-xs font-medium text-fg-muted">Password</label>
                <div className="relative flex items-center rounded-md border border-line bg-inset px-3 focus-within:border-primary">
                  <span className="material-symbols-outlined text-fg-subtle text-lg mr-2" aria-hidden="true">lock</span>
                  <input
                    id="login-password"
                    className="h-11 w-full bg-transparent text-sm text-fg placeholder:text-fg-subtle outline-none"
                    type="password"
                    autoComplete="current-password"
                    required
                    aria-invalid={error ? true : undefined}
                    aria-describedby={error ? 'login-error' : undefined}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                </div>
              </div>

              {error && (
                <p id="login-error" role="alert" className="flex items-center gap-2 text-sm text-error">
                  <span className="material-symbols-outlined text-[18px]" aria-hidden="true">error</span>
                  {error}
                </p>
              )}

              <Button type="submit" disabled={loading} className="w-full h-11">
                {loading ? 'Signing in…' : 'Sign in'}
              </Button>
            </form>
          </div>
        </div>
      </main>
    </div>
  );
}
