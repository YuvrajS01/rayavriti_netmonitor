import { useState, useEffect, useCallback, useMemo } from 'react';
import { getAlerts, getAlertCounts, acknowledgeAlert, resolveAlert } from '../api/client';
import type { Alert, AlertCounts } from '../api/types';

function severityIcon(severity: string) {
  if (severity === 'critical') return 'dangerous';
  if (severity === 'warning') return 'warning';
  return 'info';
}

function severityColors(severity: string) {
  if (severity === 'critical') return { text: 'text-error', bg: 'bg-error/10', border: 'border-error' };
  if (severity === 'warning') return { text: 'text-amber-500', bg: 'bg-amber-500/10', border: 'border-amber-500' };
  return { text: 'text-primary', bg: 'bg-primary/10', border: 'border-primary' };
}

export default function Alerts() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [counts, setCounts] = useState<AlertCounts>({ active: 0, acknowledged: 0, resolved: 0 });
  const [currentTab, setCurrentTab] = useState<'active' | 'acknowledged' | 'resolved'>('active');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (tab: string = currentTab) => {
    try {
      setError(null);
      const [alertsRes, countsRes] = await Promise.all([
        getAlerts(tab, 300),
        getAlertCounts(),
      ]);
      setAlerts(alertsRes.data || []);
      setCounts(countsRes.data || { active: 0, acknowledged: 0, resolved: 0 });
    } catch {
      setError('Failed to load alerts. Please try again.');
    } finally {
      setLoading(false);
    }
  }, [currentTab]);

  useEffect(() => { load(currentTab); }, [currentTab, load]);

  const handleAck = async (id: number) => {
    await acknowledgeAlert(id);
    load(currentTab);
  };

  const handleResolve = async (id: number) => {
    await resolveAlert(id);
    load(currentTab);
  };

  const tabs = [
    { key: 'active' as const, label: 'Active', count: counts.active },
    { key: 'acknowledged' as const, label: 'Acknowledged', count: counts.acknowledged },
    { key: 'resolved' as const, label: 'Resolved', count: counts.resolved },
  ];

  const critical = useMemo(() => alerts.filter((a) => a.severity === 'critical'), [alerts]);
  const warnings = useMemo(() => alerts.filter((a) => a.severity === 'warning'), [alerts]);
  const info = useMemo(() => alerts.filter((a) => a.severity === 'info'), [alerts]);

  return (
    <div>
      <header className="mb-12">
        <h1 className="font-headline text-4xl font-bold tracking-tight text-on-surface mb-2">SYSTEM ALERTS</h1>
        <p className="font-body text-on-surface-variant max-w-2xl">Real-time surveillance of network anomalies and node failures. Active pulse monitoring is operational.</p>
      </header>

      {/* Status Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-12">
        <div className="md:col-span-2 bg-surface-container-low p-6 rounded-xl border-l-4 border-error">
          <div className="flex justify-between items-start mb-4">
            <span className="text-error font-headline font-bold text-lg tracking-widest uppercase">Critical Events</span>
            <span className="material-symbols-outlined text-error" style={{ fontVariationSettings: "'FILL' 1" }}>error</span>
          </div>
          <div className="text-5xl font-headline font-black text-on-surface">{String(counts.active).padStart(2, '0')}</div>
          <p className="text-xs text-on-surface-variant mt-2 uppercase tracking-tighter">Requires immediate intervention</p>
        </div>
        <div className="bg-surface-container-low p-6 rounded-xl border-l-4 border-amber-500">
          <div className="flex justify-between items-start mb-4">
            <span className="text-amber-500 font-headline font-bold text-sm tracking-widest uppercase">Acknowledged</span>
            <span className="material-symbols-outlined text-amber-500">check_circle</span>
          </div>
          <div className="text-3xl font-headline font-black text-on-surface">{counts.acknowledged}</div>
        </div>
        <div className="bg-surface-container-low p-6 rounded-xl border-l-4 border-primary">
          <div className="flex justify-between items-start mb-4">
            <span className="text-primary font-headline font-bold text-sm tracking-widest uppercase">Resolved</span>
            <span className="material-symbols-outlined text-primary">done_all</span>
          </div>
          <div className="text-3xl font-headline font-black text-on-surface">{counts.resolved}</div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex flex-wrap items-center justify-between gap-4 mb-8">
        <div className="flex flex-wrap gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setCurrentTab(tab.key)}
              className={`px-4 py-2 text-xs rounded-lg border font-bold uppercase tracking-widest transition-all ${
                currentTab === tab.key
                  ? 'border-primary/40 text-primary bg-primary/5'
                  : 'border-outline-variant/20 text-on-surface-variant hover:text-primary'
              }`}
            >
              {tab.label} ({tab.count})
            </button>
          ))}
        </div>
        <p className="text-xs text-on-surface-variant">Live alerts feed</p>
      </div>

      {/* Loading State */}
      {loading && (
        <div className="flex flex-col items-center justify-center py-16">
          <span className="material-symbols-outlined text-4xl text-primary animate-pulse mb-3">hourglass_top</span>
          <p className="text-sm text-on-surface-variant uppercase tracking-widest">Loading alerts...</p>
        </div>
      )}

      {/* Error State */}
      {error && !loading && (
        <div className="bg-error/10 border border-error/30 rounded-xl p-6 text-center">
          <span className="material-symbols-outlined text-error text-3xl mb-2">error</span>
          <p className="text-sm text-error font-bold">{error}</p>
          <button onClick={() => load(currentTab)} className="mt-3 text-xs text-on-surface-variant hover:text-primary transition-colors underline">
            Retry
          </button>
        </div>
      )}

      {/* Alerts List */}
      {!loading && !error && (
        <div className="space-y-4">
          {currentTab === 'active' && critical.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-error/20" />
                <span className="font-headline text-error font-bold tracking-widest text-sm uppercase">Priority: Critical</span>
                <span className="h-px flex-1 bg-error/20" />
              </div>
              {critical.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && warnings.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-amber-500/20" />
                <span className="font-headline text-amber-500 font-bold tracking-widest text-sm uppercase">Status: Warning</span>
                <span className="h-px flex-1 bg-amber-500/20" />
              </div>
              {warnings.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab === 'active' && info.length > 0 && (
            <>
              <div className="flex items-center gap-4 py-4">
                <span className="h-px flex-1 bg-primary/20" />
                <span className="font-headline text-primary font-bold tracking-widest text-sm uppercase">Logs: Information</span>
                <span className="h-px flex-1 bg-primary/20" />
              </div>
              {info.map((alert) => <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />)}
            </>
          )}

          {currentTab !== 'active' && alerts.map((alert) => (
            <AlertItem key={alert.id} alert={alert} onAck={handleAck} onResolve={handleResolve} />
          ))}

          {alerts.length === 0 && (
            <div className="text-sm text-on-surface-variant text-center py-12">No alerts in this view</div>
          )}
        </div>
      )}
    </div>
  );
}

function AlertItem({ alert, onAck, onResolve }: { alert: Alert; onAck: (id: number) => void; onResolve: (id: number) => void }) {
  const sc = severityColors(alert.severity);
  const deviceName = alert.deviceName || `Device ${alert.deviceId}`;

  return (
    <div className={`group bg-surface-container-low rounded-xl border-l-[6px] ${sc.border} p-5 flex flex-col md:flex-row md:items-center justify-between gap-6 transition-all hover:bg-surface-container-high`}>
      <div className="flex items-start gap-4">
        <div className={`${sc.bg} p-3 rounded-lg`}>
          <span className={`material-symbols-outlined ${sc.text}`} style={{ fontVariationSettings: "'FILL' 1" }}>
            {severityIcon(alert.severity)}
          </span>
        </div>
        <div>
          <div className="flex items-center gap-3 mb-1 flex-wrap">
            <h3 className="font-headline font-bold text-on-surface tracking-tight uppercase">{alert.message}</h3>
            <span className={`text-[10px] ${sc.bg} ${sc.text} px-2 py-0.5 font-bold rounded`}>
              {alert.severity.toUpperCase()}
            </span>
          </div>
          <p className="text-sm text-on-surface-variant">{deviceName}</p>
          <span className="text-[10px] font-mono text-on-surface-variant uppercase mt-2 block">
            {new Date(alert.createdAt).toLocaleString()} • {alert.status}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-4">
        {alert.status === 'active' && (
          <button onClick={() => onAck(alert.id)} className={`px-6 py-2 border-2 ${sc.border} ${sc.text} font-headline font-bold text-xs uppercase hover:bg-surface-container-highest transition-all active:scale-95`}>
            ACKNOWLEDGE
          </button>
        )}
        {alert.status === 'acknowledged' && (
          <button onClick={() => onResolve(alert.id)} className="px-6 py-2 border-2 border-primary text-primary font-headline font-bold text-xs uppercase hover:bg-primary hover:text-on-primary transition-all active:scale-95">
            RESOLVE
          </button>
        )}
        {alert.status === 'resolved' && (
          <span className="text-[10px] uppercase font-bold tracking-widest text-on-surface-variant">Resolved</span>
        )}
      </div>
    </div>
  );
}
