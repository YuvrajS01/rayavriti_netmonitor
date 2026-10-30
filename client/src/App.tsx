import { Suspense, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import ErrorBoundary from './components/ErrorBoundary';
import PageLoader from './components/ui/PageLoader';
import Button from './components/ui/Button';
import { lazyWithRetry } from './utils/lazyWithRetry';
import { SocketProvider } from './hooks/useSocket';
import { setMaintenanceHandler } from './api/http';
import { restoreSession } from './api/auth';
import { useIsAuthenticated, useSessionStatus } from './store/session';
import { ToastProvider } from './components/ui/Toast';
import { QueryClientProvider } from '@tanstack/react-query';
import { queryClient } from './api/queryClient';
import { useLiveQueryBridge } from './hooks/useLiveQueryBridge';

import Layout from './components/Layout';
import SettingsLayout from './components/layout/SettingsLayout';
import { DiscoveryHub, LegacyRedirect, SectionHub } from './components/layout/SectionHub';
import { LEGACY_REDIRECTS } from './navigation';

const CAMPUS_VIEWS = ['tree', 'map'] as const;
const FLOOR_VIEWS = ['floor'] as const;

const Login = lazyWithRetry(() => import('./pages/Login'));
const Dashboard = lazyWithRetry(() => import('./pages/Dashboard'));
const Devices = lazyWithRetry(() => import('./pages/Devices'));
const NetworkTopology = lazyWithRetry(() => import('./pages/NetworkTopology'));
const Sensors = lazyWithRetry(() => import('./pages/Sensors'));
const FlowAnalysis = lazyWithRetry(() => import('./pages/FlowAnalysis'));
const PacketCapture = lazyWithRetry(() => import('./pages/PacketCapture'));
const AIHealth = lazyWithRetry(() => import('./pages/AIHealth'));
const Alerts = lazyWithRetry(() => import('./pages/Alerts'));
const Reports = lazyWithRetry(() => import('./pages/Reports'));
const Logs = lazyWithRetry(() => import('./pages/Logs'));
const Settings = lazyWithRetry(() => import('./pages/Settings'));
const Campus = lazyWithRetry(() => import('./pages/Campus'));
const LocationManager = lazyWithRetry(() => import('./pages/LocationManager'));
const Incidents = lazyWithRetry(() => import('./pages/Incidents'));
const IncidentDetail = lazyWithRetry(() => import('./pages/IncidentDetail'));
const Contacts = lazyWithRetry(() => import('./pages/Contacts'));
const StatusPageAdmin = lazyWithRetry(() => import('./pages/StatusPageAdmin'));
const Maintenance = lazyWithRetry(() => import('./pages/Maintenance'));
const UserManagement = lazyWithRetry(() => import('./pages/UserManagement'));
const ReportBuilder = lazyWithRetry(() => import('./pages/ReportBuilder'));
const Discovery = lazyWithRetry(() => import('./pages/Discovery'));
const ServiceTemplates = lazyWithRetry(() => import('./pages/ServiceTemplates'));
const BulkImport = lazyWithRetry(() => import('./pages/BulkImport'));
const ISP = lazyWithRetry(() => import('./pages/ISP'));
const BackupPage = lazyWithRetry(() => import('./pages/Backup'));
const RemoteMonitoring = lazyWithRetry(() => import('./pages/RemoteMonitoring'));
const SystemMaintenance = lazyWithRetry(() => import('./pages/SystemMaintenance'));
const SystemSettings = lazyWithRetry(() => import('./pages/SystemSettings'));
const NotFound = lazyWithRetry(() => import('./pages/NotFound'));

function FullScreenStatus({ message, children }: { message: string; children?: React.ReactNode }) {
  return (
    <div role="status" aria-live="polite" className="flex items-center justify-center min-h-screen bg-canvas">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined animate-spin text-3xl text-fg-muted" aria-hidden="true">progress_activity</span>
        <p className="text-sm text-fg-muted">{message}</p>
        {children}
      </div>
    </div>
  );
}

/** Waits for the session, then renders its children (default: the app shell with the routed page). */
function RequireAuth({ children }: { children?: React.ReactNode }) {
  const status = useSessionStatus();

  if (status === 'restoring') return <FullScreenStatus message="Verifying session..." />;
  if (status === 'unreachable') {
    return (
      <FullScreenStatus message="Unable to reach the server">
        <Button variant="secondary" size="sm" icon="refresh" onClick={() => void restoreSession()} className="mt-2">
          Retry
        </Button>
      </FullScreenStatus>
    );
  }
  if (status !== 'authenticated') return <Navigate to="/login" replace />;
  return children ?? <Layout />;
}

/** Routes maintenance-mode redirects from the HTTP layer through the router. */
function MaintenanceRedirect() {
  const navigate = useNavigate();
  useEffect(() => {
    setMaintenanceHandler(() => { void navigate('/system-maintenance'); });
  }, [navigate]);
  return null;
}

function AppRoutes() {
  const isAuth = useIsAuthenticated();

  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        <Route path="/login" element={isAuth ? <Navigate to="/" replace /> : <Login />} />
        <Route path="/system-maintenance" element={<SystemMaintenance />} />

        <Route element={<RequireAuth />}>
          {/* Monitor */}
          <Route index element={<Dashboard />} />
          <Route path="alerts" element={<SectionHub itemId="alerts" />}>
            <Route index element={<Alerts />} />
            <Route path="incidents" element={<Incidents />} />
            <Route path="incidents/:id" element={<IncidentDetail />} />
            <Route path="maintenance" element={<Maintenance />} />
          </Route>
          <Route path="topology" element={<SectionHub itemId="topology" />}>
            <Route index element={<NetworkTopology />} />
            <Route path="campus" element={<Campus views={CAMPUS_VIEWS} />} />
            <Route path="floor-plans" element={<Campus views={FLOOR_VIEWS} />} />
          </Route>

          {/* Inventory */}
          <Route path="devices" element={<Devices />} />
          <Route path="devices/sensors" element={<Sensors />} />
          <Route path="isp" element={<ISP />} />
          <Route path="remote" element={<RemoteMonitoring />} />

          {/* Analyze */}
          <Route path="flows" element={<SectionHub itemId="flows" />}>
            <Route index element={<FlowAnalysis />} />
            <Route path="capture" element={<PacketCapture />} />
          </Route>
          <Route path="logs" element={<Logs />} />
          <Route path="reports" element={<SectionHub itemId="reports" />}>
            <Route index element={<Reports />} />
            <Route path="library" element={<ReportBuilder section="generated" />} />
            <Route path="scheduled" element={<ReportBuilder section="scheduled" />} />
          </Route>
          <Route path="ai-health" element={<AIHealth />} />

          {/* Configure */}
          <Route path="settings" element={<SettingsLayout />}>
            <Route index element={<Settings />} />
            <Route path="users" element={<UserManagement />} />
            <Route path="contacts" element={<Contacts />} />
            <Route path="locations" element={<LocationManager />} />
            <Route path="templates" element={<ServiceTemplates />} />
            <Route path="discovery" element={<DiscoveryHub />}>
              <Route index element={<Discovery />} />
              <Route path="import" element={<BulkImport />} />
            </Route>
            <Route path="status-page" element={<StatusPageAdmin />} />
            <Route path="backup" element={<BackupPage />} />
            <Route path="system" element={<SystemSettings />} />
          </Route>

          {/* v5.0 URLs keep working. */}
          {LEGACY_REDIRECTS.map(({ from, to }) => (
            <Route key={from} path={from} element={<LegacyRedirect to={to} />} />
          ))}

          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </Suspense>
  );
}

function LiveQueryBridge() {
  useLiveQueryBridge();
  return null;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <SocketProvider>
          <LiveQueryBridge />
          <ToastProvider>
            <BrowserRouter>
              <MaintenanceRedirect />
              <AppRoutes />
            </BrowserRouter>
          </ToastProvider>
        </SocketProvider>
      </ErrorBoundary>
    </QueryClientProvider>
  );
}
