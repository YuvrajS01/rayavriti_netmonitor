import { Suspense, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import ErrorBoundary from './components/ErrorBoundary';
import PageLoader from './components/ui/PageLoader';
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
const SecurityInventory = lazyWithRetry(() => import('./pages/SecurityInventory'));
const NotFound = lazyWithRetry(() => import('./pages/NotFound'));

function FullScreenStatus({ message, children }: { message: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-center min-h-screen bg-background">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
        <p className="text-xs text-on-surface-variant uppercase tracking-wide">{message}</p>
        {children}
      </div>
    </div>
  );
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const status = useSessionStatus();

  if (status === 'restoring') return <FullScreenStatus message="Verifying session..." />;
  if (status === 'unreachable') {
    return (
      <FullScreenStatus message="Unable to reach the server">
        <button
          onClick={() => void restoreSession()}
          className="mt-2 text-xs text-on-surface-variant hover:text-primary transition-colors underline"
        >
          Retry
        </button>
      </FullScreenStatus>
    );
  }
  if (status !== 'authenticated') return <Navigate to="/login" replace />;
  return <Layout>{children}</Layout>;
}

/** Routes maintenance-mode redirects from the HTTP layer through the router. */
function MaintenanceRedirect() {
  const navigate = useNavigate();
  useEffect(() => {
    setMaintenanceHandler(() => navigate('/system-maintenance'));
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
        <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
        <Route path="/devices" element={<ProtectedRoute><Devices /></ProtectedRoute>} />
		<Route path="/inventory/cameras" element={<ProtectedRoute><SecurityInventory profile="camera" /></ProtectedRoute>} />
		<Route path="/inventory/biometrics" element={<ProtectedRoute><SecurityInventory profile="biometric" /></ProtectedRoute>} />
        <Route path="/devices/topology" element={<ProtectedRoute><NetworkTopology /></ProtectedRoute>} />
        <Route path="/sensors" element={<ProtectedRoute><Sensors /></ProtectedRoute>} />
        <Route path="/flows" element={<ProtectedRoute><FlowAnalysis /></ProtectedRoute>} />
        <Route path="/capture" element={<ProtectedRoute><PacketCapture /></ProtectedRoute>} />
        <Route path="/ai-health" element={<ProtectedRoute><AIHealth /></ProtectedRoute>} />
        <Route path="/alerts" element={<ProtectedRoute><Alerts /></ProtectedRoute>} />
        <Route path="/reports" element={<ProtectedRoute><Reports /></ProtectedRoute>} />
        <Route path="/reports/builder" element={<ProtectedRoute><ReportBuilder /></ProtectedRoute>} />
        <Route path="/logs" element={<ProtectedRoute><Logs /></ProtectedRoute>} />
        <Route path="/campus" element={<ProtectedRoute><Campus /></ProtectedRoute>} />
        <Route path="/incidents" element={<ProtectedRoute><Incidents /></ProtectedRoute>} />
        <Route path="/incidents/:id" element={<ProtectedRoute><IncidentDetail /></ProtectedRoute>} />
        <Route path="/maintenance" element={<ProtectedRoute><Maintenance /></ProtectedRoute>} />
        <Route path="/discovery" element={<ProtectedRoute><Discovery /></ProtectedRoute>} />
        <Route path="/service-templates" element={<ProtectedRoute><ServiceTemplates /></ProtectedRoute>} />
        <Route path="/import" element={<ProtectedRoute><BulkImport /></ProtectedRoute>} />
        <Route path="/isp" element={<ProtectedRoute><ISP /></ProtectedRoute>} />
		<Route path="/remote" element={<ProtectedRoute><RemoteMonitoring /></ProtectedRoute>} />
        <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
        <Route path="/settings/locations" element={<ProtectedRoute><LocationManager /></ProtectedRoute>} />
        <Route path="/settings/contacts" element={<ProtectedRoute><Contacts /></ProtectedRoute>} />
        <Route path="/settings/status-page" element={<ProtectedRoute><StatusPageAdmin /></ProtectedRoute>} />
        <Route path="/settings/users" element={<ProtectedRoute><UserManagement /></ProtectedRoute>} />
        <Route path="/settings/backup" element={<ProtectedRoute><BackupPage /></ProtectedRoute>} />
        <Route path="*" element={<ProtectedRoute><NotFound /></ProtectedRoute>} />
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
