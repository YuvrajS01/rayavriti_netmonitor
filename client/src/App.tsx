import { lazy, Suspense, useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import ErrorBoundary from './components/ErrorBoundary';
import { Provider, useSelector, useDispatch } from 'react-redux';
import { store, type RootState } from './store';
import { SocketProvider } from './hooks/useSocket';
import { clearCredentials } from './store/authSlice';
import { api } from './api/http';

import Layout from './components/Layout';

const Login = lazy(() => import('./pages/Login'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Devices = lazy(() => import('./pages/Devices'));
const Sensors = lazy(() => import('./pages/Sensors'));
const FlowAnalysis = lazy(() => import('./pages/FlowAnalysis'));
const PacketCapture = lazy(() => import('./pages/PacketCapture'));
const AIHealth = lazy(() => import('./pages/AIHealth'));
const Alerts = lazy(() => import('./pages/Alerts'));
const Reports = lazy(() => import('./pages/Reports'));
const Settings = lazy(() => import('./pages/Settings'));

function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
        <p className="text-xs text-on-surface-variant uppercase tracking-widest">Loading...</p>
      </div>
    </div>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuth = useSelector((s: RootState) => s.auth.isAuthenticated);
  const dispatch = useDispatch();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    if (!isAuth) {
      setChecking(false);
      return;
    }
    api.get('/auth/me')
      .catch(() => {
        dispatch(clearCredentials());
      })
      .finally(() => setChecking(false));
  }, [isAuth, dispatch]);

  if (!isAuth) return <Navigate to="/login" replace />;
  if (checking) {
    return (
      <Layout>
        <div className="flex items-center justify-center min-h-[60vh]">
          <div className="flex flex-col items-center gap-3">
            <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
            <p className="text-xs text-on-surface-variant uppercase tracking-widest">Verifying session...</p>
          </div>
        </div>
      </Layout>
    );
  }
  return <Layout>{children}</Layout>;
}

function AppRoutes() {
  const isAuth = useSelector((s: RootState) => s.auth.isAuthenticated);

  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        <Route path="/login" element={isAuth ? <Navigate to="/" replace /> : <Login />} />
        <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
        <Route path="/devices" element={<ProtectedRoute><Devices /></ProtectedRoute>} />
        <Route path="/sensors" element={<ProtectedRoute><Sensors /></ProtectedRoute>} />
        <Route path="/flows" element={<ProtectedRoute><FlowAnalysis /></ProtectedRoute>} />
        <Route path="/capture" element={<ProtectedRoute><PacketCapture /></ProtectedRoute>} />
        <Route path="/ai-health" element={<ProtectedRoute><AIHealth /></ProtectedRoute>} />
        <Route path="/alerts" element={<ProtectedRoute><Alerts /></ProtectedRoute>} />
        <Route path="/reports" element={<ProtectedRoute><Reports /></ProtectedRoute>} />
        <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <ErrorBoundary>
        <SocketProvider>
          <BrowserRouter>
            <AppRoutes />
          </BrowserRouter>
        </SocketProvider>
      </ErrorBoundary>
    </Provider>
  );
}
