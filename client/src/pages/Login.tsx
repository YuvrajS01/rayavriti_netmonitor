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
      dispatch(setCredentials({ token: result.data.token, user: result.data.user }));
      navigate('/');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Login failed';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-background text-on-surface font-body min-h-screen flex items-center justify-center overflow-hidden">
      {/* Atmospheric Background */}
      <div className="fixed inset-0 z-0">
        <div className="absolute inset-0 bg-gradient-to-tr from-surface-container-lowest via-surface-dim to-surface-container-low" />
        <div className="absolute inset-0 particle-bg opacity-10" />
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/5 blur-[120px] rounded-full" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[50%] h-[50%] bg-primary/5 blur-[150px] rounded-full" />
      </div>

      <main className="relative z-10 w-full max-w-md px-6 py-12">
        {/* Brand */}
        <div className="flex flex-col items-center mb-12 space-y-2">
          <div className="w-16 h-16 mb-4 flex items-center justify-center">
            <svg fill="none" height="64" viewBox="0 0 100 100" width="64">
              <path className="drop-shadow-[0_0_8px_rgba(217,253,58,0.5)]" d="M50 5L95 27.5V72.5L50 95L5 72.5V27.5L50 5Z" stroke="#d9fd3a" strokeWidth="2" />
              <path d="M50 20L75 32.5V67.5L50 80L25 67.5V32.5L50 20Z" fill="#d9fd3a" fillOpacity="0.1" />
              <circle cx="50" cy="50" fill="#d9fd3a" r="10" />
            </svg>
          </div>
          <h1 className="font-headline text-4xl font-black tracking-tighter text-primary">Rayavriti NetMonitor+</h1>
          <p className="font-label text-xs tracking-[0.4em] text-on-surface-variant uppercase">Network Surveillance Interface</p>
        </div>

        {/* Login Card */}
        <div className="bg-surface-container-high/80 backdrop-blur-xl rounded-xl p-8 neon-glow border border-primary/10">
          <div className="mb-8">
            <h2 className="font-headline text-lg text-on-surface tracking-wide">Account Login</h2>
            <div className="h-0.5 w-12 bg-primary mt-2" />
          </div>

          <form className="space-y-6" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <label className="font-label text-[10px] tracking-widest text-on-surface-variant uppercase ml-1">Username</label>
              <div className="relative flex items-center bg-surface-container-highest rounded-lg px-4 py-3 border border-transparent transition-all duration-300 geometric-input">
                <span className="material-symbols-outlined text-outline text-lg mr-3">person</span>
                <input
                  className="bg-transparent border-none p-0 w-full text-on-surface placeholder:text-outline focus:ring-0 text-sm tracking-tight font-body outline-none"
                  placeholder="Enter identification string..."
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
            </div>

            <div className="space-y-2">
              <label className="font-label text-[10px] tracking-widest text-on-surface-variant uppercase ml-1">Password</label>
              <div className="relative flex items-center bg-surface-container-highest rounded-lg px-4 py-3 border border-transparent transition-all duration-300 geometric-input">
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
              className="w-full bg-primary hover:bg-primary-container text-on-primary font-headline font-bold py-4 rounded-lg transition-all duration-300 flex items-center justify-center space-x-2 group active:scale-95 shadow-[0_4px_20px_rgba(217,253,58,0.2)] disabled:opacity-50"
            >
              <span className="tracking-widest">{loading ? 'Signing In...' : 'Sign In'}</span>
              <span className="material-symbols-outlined transition-transform duration-300 group-hover:translate-x-1">arrow_forward</span>
            </button>
          </form>

          {error && <div className="text-error text-xs mt-4">{error}</div>}
        </div>

        {/* Footer */}
        <div className="mt-8 text-center space-y-4">
          <div className="flex justify-center items-center space-x-6">
            <div className="flex items-center space-x-2">
              <div className="w-2 h-2 rounded-full bg-primary animate-pulse" />
              <span className="font-label text-[10px] text-primary tracking-widest uppercase">System Online</span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="material-symbols-outlined text-[14px] text-outline">verified_user</span>
              <span className="font-label text-[10px] text-on-surface-variant tracking-widest uppercase">TLS Encrypted</span>
            </div>
          </div>
          <p className="font-label text-[9px] text-outline uppercase tracking-[0.2em] leading-relaxed">
            Unauthorized access to Rayavriti NetMonitor+ is strictly prohibited.<br />
            All connection attempts are logged and monitored.
          </p>
        </div>
      </main>
    </div>
  );
}
