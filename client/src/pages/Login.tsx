import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDispatch } from 'react-redux';
import { setCredentials } from '../store/authSlice';
import { login } from '../api/client';
import LockupColor from '../assets/brand/Lockup-color.svg';
import IconColor from '../assets/brand/Icon-color.svg';

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
    <div className="bg-surface text-on-surface font-body min-h-screen flex">
      {/* Left Panel — Brand */}
      <div
        className="hidden lg:flex lg:w-1/2 bg-surface-dim items-center justify-center"
        style={{
          backgroundImage: 'radial-gradient(circle, oklch(22% 0.016 150) 1px, transparent 1px)',
          backgroundSize: '24px 24px',
        }}
      >
        <img src={LockupColor} alt="Rayavriti" className="max-w-xs" />
      </div>

      {/* Right Panel — Form */}
      <main className="w-full lg:w-1/2 flex items-center justify-center px-6 py-12">
        <div className="w-full max-w-md">
          {/* Mobile brand icon */}
          <img src={IconColor} alt="Rayavriti" className="w-10 h-10 mb-6 lg:hidden" />

          {/* Login Card */}
          <div className="bg-surface-container-low rounded-xl p-8 w-full max-w-md border border-outline-variant/20">
            <div className="mb-8">
              <h2 className="font-headline text-2xl font-semibold text-on-surface">Sign in</h2>
            </div>

            <form className="space-y-6" onSubmit={handleSubmit}>
              <div className="space-y-2">
                <label className="font-label text-xs tracking-wide text-on-surface-variant ml-1">Username</label>
                <div className="relative flex items-center bg-surface-container rounded-lg px-4 py-3 border border-outline-variant/20 text-on-surface placeholder:text-outline focus-within:border-primary focus-within:ring-1 focus-within:ring-primary/30">
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
                <div className="relative flex items-center bg-surface-container rounded-lg px-4 py-3 border border-outline-variant/20 text-on-surface placeholder:text-outline focus-within:border-primary focus-within:ring-1 focus-within:ring-primary/30">
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
                className="bg-primary text-on-primary font-body font-medium py-3 rounded-md w-full hover:bg-primary-dim transition-colors duration-200 flex items-center justify-center disabled:opacity-50"
              >
                <span>{loading ? 'Signing In...' : 'Sign In'}</span>
              </button>
            </form>

            {error && <div className="text-error text-sm mt-4">{error}</div>}
          </div>
        </div>
      </main>
    </div>
  );
}
