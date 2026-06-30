import { lazy, Suspense, useEffect, useState, useRef } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import ErrorBoundary from './components/ErrorBoundary';
import { Provider, useSelector, useDispatch } from 'react-redux';
import { store, type RootState } from './store';
import { SocketProvider } from './hooks/useSocket';
import { clearCredentials, setPermissions } from './store/authSlice';
import { v1 } from './api/http';
import { ToastProvider } from './components/ui/Toast';

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
const Campus = lazy(() => import('./pages/Campus'));
const LocationManager = lazy(() => import('./pages/LocationManager'));
const Incidents = lazy(() => import('./pages/Incidents'));
const IncidentDetail = lazy(() => import('./pages/IncidentDetail'));
const Contacts = lazy(() => import('./pages/Contacts'));
const StatusPageAdmin = lazy(() => import('./pages/StatusPageAdmin'));
const Maintenance = lazy(() => import('./pages/Maintenance'));
const UserManagement = lazy(() => import('./pages/UserManagement'));
const ReportBuilder = lazy(() => import('./pages/ReportBuilder'));
const Discovery = lazy(() => import('./pages/Discovery'));
const ServiceTemplates = lazy(() => import('./pages/ServiceTemplates'));
const BulkImport = lazy(() => import('./pages/BulkImport'));
const ISP = lazy(() => import('./pages/ISP'));
const NotFound = lazy(() => import('./pages/NotFound'));

function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
        <p className="text-xs text-on-surface-variant uppercase tracking-wide">Loading...</p>
      </div>
    </div>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuth = useSelector((s: RootState) => s.auth.isAuthenticated);
  const dispatch = useDispatch();
  const sessionCheckedRef = useRef(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    if (!isAuth || sessionCheckedRef.current) {
      setChecking(false);
      return;
    }
    v1.get('/auth/me')
      .then(() => {
        sessionCheckedRef.current = true;
        return v1.get('/auth/permissions').catch(() => null);
      })
      .then((res: { data: unknown } | null) => {
        if (!res) return;
        const perms = (res.data as { data?: { permissions?: string[] } })?.data?.permissions;
        if (perms) dispatch(setPermissions(perms));
      })
      .catch(() => {
        sessionCheckedRef.current = true;
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
            <p className="text-xs text-on-surface-variant uppercase tracking-wide">Verifying session...</p>
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
        <Route path="/reports/builder" element={<ProtectedRoute><ReportBuilder /></ProtectedRoute>} />
        <Route path="/campus" element={<ProtectedRoute><Campus /></ProtectedRoute>} />
        <Route path="/incidents" element={<ProtectedRoute><Incidents /></ProtectedRoute>} />
        <Route path="/incidents/:id" element={<ProtectedRoute><IncidentDetail /></ProtectedRoute>} />
        <Route path="/maintenance" element={<ProtectedRoute><Maintenance /></ProtectedRoute>} />
        <Route path="/discovery" element={<ProtectedRoute><Discovery /></ProtectedRoute>} />
        <Route path="/service-templates" element={<ProtectedRoute><ServiceTemplates /></ProtectedRoute>} />
        <Route path="/import" element={<ProtectedRoute><BulkImport /></ProtectedRoute>} />
        <Route path="/isp" element={<ProtectedRoute><ISP /></ProtectedRoute>} />
        <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
        <Route path="/settings/locations" element={<ProtectedRoute><LocationManager /></ProtectedRoute>} />
        <Route path="/settings/contacts" element={<ProtectedRoute><Contacts /></ProtectedRoute>} />
        <Route path="/settings/status-page" element={<ProtectedRoute><StatusPageAdmin /></ProtectedRoute>} />
        <Route path="/settings/users" element={<ProtectedRoute><UserManagement /></ProtectedRoute>} />
        <Route path="*" element={<ProtectedRoute><NotFound /></ProtectedRoute>} />
      </Routes>
    </Suspense>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <ErrorBoundary>
        <SocketProvider>
          <ToastProvider>
            <BrowserRouter>
              <AppRoutes />
            </BrowserRouter>
          </ToastProvider>
        </SocketProvider>
      </ErrorBoundary>
    </Provider>
  );
}
