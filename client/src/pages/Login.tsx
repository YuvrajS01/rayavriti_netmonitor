import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDispatch } from 'react-redux';
import { setCredentials } from '../store/authSlice';
import { login } from '../api/client';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const dispatch = useDispatch();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const result = await login(username.trim(), password);
      const u = result.data.user;
      dispatch(setCredentials({
        token: result.data.token,
        user: { id: u.id, username: u.username, role: u.role, permissions: u.permissions },
      }));
      navigate('/');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Login failed';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-background text-on-surface font-body min-h-screen flex items-center justify-center">
      <main className="w-full max-w-md px-6 py-12">
        {/* Brand */}
        <div className="flex flex-col items-center mb-12 space-y-2">
          <div className="w-16 h-16 mb-4 flex items-center justify-center">
            <svg fill="none" height="64" viewBox="0 0 100 100" width="64">
              <path d="M50 5L95 27.5V72.5L50 95L5 72.5V27.5L50 5Z" stroke="#d9fd3a" strokeWidth="2" />
              <path d="M50 20L75 32.5V67.5L50 80L25 67.5V32.5L50 20Z" fill="#d9fd3a" fillOpacity="0.1" />
              <circle cx="50" cy="50" fill="#d9fd3a" r="10" />
            </svg>
          </div>
          <h1 className="font-headline text-4xl font-bold tracking-tighter text-primary">Rayavriti NetMonitor+</h1>
          <p className="font-label text-xs tracking-wide text-on-surface-variant uppercase">Network Monitoring</p>
        </div>

        {/* Login Card */}
        <div className="bg-surface-container-high rounded-lg p-8 border border-outline-variant/20">
          <div className="mb-8">
            <h2 className="font-headline text-lg text-on-surface tracking-wide">Sign in</h2>
            <div className="h-0.5 w-12 bg-primary mt-2" />
          </div>

          <form className="space-y-6" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <label className="font-label text-xs tracking-wide text-on-surface-variant ml-1">Username</label>
              <div className="relative flex items-center bg-surface-container-highest rounded-lg px-4 py-3 border border-outline-variant/20 transition-[border-color] duration-300">
                <span className="material-symbols-outlined text-outline text-lg mr-3">person</span>
                <input
                  className="bg-transparent border-none p-0 w-full text-on-surface placeholder:text-outline focus:ring-0 text-sm tracking-tight font-body outline-none"
                  placeholder="Username"
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className="font-label text-xs tracking-wide text-on-surface-variant ml-1">Password</label>
              <div className="relative flex items-center bg-surface-container-highest rounded-lg px-4 py-3 border border-outline-variant/20 transition-[border-color] duration-300">
                <span className="material-symbols-outlined text-outline text-lg mr-3">lock</span>
                <input
                  className="bg-transparent border-none p-0 w-full text-on-surface placeholder:text-outline focus:ring-0 text-sm tracking-tight font-body outline-none"
                  placeholder="••••••••••••"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full bg-primary hover:bg-primary-container text-on-primary font-headline font-bold py-4 rounded-md transition-[background-color] duration-300 flex items-center justify-center disabled:opacity-50"
            >
              <span className="tracking-wider">{loading ? 'Signing In...' : 'Sign In'}</span>
            </button>
          </form>

          {error && <div className="text-error text-xs mt-4">{error}</div>}
        </div>
      </main>
    </div>
  );
}
