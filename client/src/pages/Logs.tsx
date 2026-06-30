import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Button from '../components/ui/Button';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import { createVerboseSession, getLogs, getLogStats, getVerboseSessions, logsExportUrl, stopVerboseSession, type LogEvent, type LogFilters, type LogStats, type VerboseSession } from '../api/logs';
import { useToast } from '../components/ui/useToast';

const LEVELS = ['', 'trace', 'debug', 'info', 'warn', 'error'];
const COMPONENTS = ['', 'http', 'db', 'audit', 'collector', 'collector.snmp', 'websocket', 'scheduler'];

const defaultStats: LogStats = { total: 0, byLevel: {}, byComponent: {}, errors: 0, slowRequests: 0, slowQueries: 0 };

function dateTimeLocalHoursAgo(hours: number) {
  const d = new Date(Date.now() - hours * 60 * 60 * 1000);
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

function toIsoLocal(value: string) {
  return value ? new Date(value).toISOString() : undefined;
}

function fmtTime(ts: string) {
  return new Date(ts).toLocaleString([], { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function levelClass(level: string) {
  if (level === 'error' || level === 'fatal') return 'bg-error/15 text-error';
  if (level === 'warn') return 'bg-tertiary/15 text-tertiary';
  if (level === 'debug' || level === 'trace') return 'bg-primary/12 text-primary';
  return 'bg-surface-container-highest text-on-surface-variant';
}

export default function Logs() {
  const { addToast } = useToast();
  const [events, setEvents] = useState<LogEvent[]>([]);
  const [stats, setStats] = useState<LogStats>(defaultStats);
  const [sessions, setSessions] = useState<VerboseSession[]>([]);
  const [selected, setSelected] = useState<LogEvent | null>(null);
  const [fetchSeq, setFetchSeq] = useState(1);
  const [doneSeq, setDoneSeq] = useState(0);
  const loading = fetchSeq !== doneSeq;
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [filters, setFilters] = useState({ from: dateTimeLocalHoursAgo(6), to: '', level: '', component: '', q: '', request_id: '', device_id: '' });
  const [verbose, setVerbose] = useState({ level: 'debug' as 'debug' | 'trace', component: 'collector', deviceIds: '', userIds: '', durationMinutes: 30, reason: '' });

  const query = useMemo<LogFilters>(() => ({
    from: toIsoLocal(filters.from),
    to: toIsoLocal(filters.to),
    level: filters.level,
    component: filters.component,
    q: filters.q,
    request_id: filters.request_id,
    device_id: filters.device_id,
    limit: 200,
  }), [filters]);

  const fetchRef = useRef(0);

  const load = useCallback(async (fetchId: number) => {
    try {
      const [logsRes, statsRes, sessionsRes] = await Promise.all([
        getLogs(query),
        getLogStats(query),
        getVerboseSessions(),
      ]);
      if (fetchRef.current !== fetchId) return;
      setEvents(logsRes.data);
      setStats(statsRes.data || defaultStats);
      setSessions(sessionsRes.data.sessions || []);
    } catch {
      if (fetchRef.current !== fetchId) return;
      addToast('Unable to load logs', 'error');
    } finally {
      if (fetchRef.current === fetchId) setDoneSeq(fetchId);
    }
  }, [query, addToast]);

  useEffect(() => {
    fetchRef.current += 1;
    setFetchSeq(fetchRef.current);
    const id = fetchRef.current;
    void load(id);
    return () => { fetchRef.current += 1; };
  }, [load]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = window.setInterval(() => {
      fetchRef.current += 1;
      setFetchSeq(fetchRef.current);
      void load(fetchRef.current);
    }, 10_000);
    return () => window.clearInterval(id);
  }, [autoRefresh, load]);

  const createSession = async () => {
    if (!verbose.reason.trim()) {
      addToast('Reason is required', 'error');
      return;
    }
    try {
      await createVerboseSession({
        level: verbose.level,
        components: verbose.component ? [verbose.component] : [],
        deviceIds: verbose.deviceIds.split(',').map((v) => Number(v.trim())).filter(Boolean),
        userIds: verbose.userIds.split(',').map((v) => v.trim()).filter(Boolean),
        reason: verbose.reason.trim(),
        durationMinutes: verbose.durationMinutes,
      });
      setVerbose((v) => ({ ...v, reason: '' }));
      addToast('Verbose logging enabled', 'success');
      fetchRef.current += 1;
      await load(fetchRef.current);
    } catch {
      addToast('Unable to enable verbose logging', 'error');
    }
  };

  const stopSession = async (id: number) => {
    try {
      await stopVerboseSession(id);
      addToast('Verbose session stopped', 'success');
      fetchRef.current += 1;
      await load(fetchRef.current);
    } catch {
      addToast('Unable to stop verbose session', 'error');
    }
  };

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Logs"
        subtitle="Search operational events and enable scoped verbose diagnostics."
        action={
          <div className="flex items-center gap-3">
            <Button variant={autoRefresh ? 'primary' : 'secondary'} icon="sync" onClick={() => setAutoRefresh((v) => !v)}>
              {autoRefresh ? 'Live' : 'Manual'}
            </Button>
            <a href={logsExportUrl(query)} className="font-headline font-bold text-sm uppercase tracking-wide rounded-md px-5 py-2.5 min-h-11 inline-flex items-center gap-2 bg-surface-container-highest text-on-surface border border-outline-variant/30 hover:bg-surface-container-high">
              <span className="material-symbols-outlined text-lg">download</span>
              Export
            </a>
          </div>
        }
      />

      <Card variant="low" className="p-4">
        <div className="grid grid-cols-1 md:grid-cols-7 gap-3">
          <input type="datetime-local" value={filters.from} onChange={(e) => setFilters((f) => ({ ...f, from: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
          <input type="datetime-local" value={filters.to} onChange={(e) => setFilters((f) => ({ ...f, to: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
          <select value={filters.level} onChange={(e) => setFilters((f) => ({ ...f, level: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm">
            {LEVELS.map((l) => <option key={l} value={l}>{l || 'All levels'}</option>)}
          </select>
          <select value={filters.component} onChange={(e) => setFilters((f) => ({ ...f, component: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm">
            {COMPONENTS.map((c) => <option key={c} value={c}>{c || 'All components'}</option>)}
          </select>
          <input placeholder="Request ID" value={filters.request_id} onChange={(e) => setFilters((f) => ({ ...f, request_id: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
          <input placeholder="Device ID" value={filters.device_id} onChange={(e) => setFilters((f) => ({ ...f, device_id: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
          <input placeholder="Search" value={filters.q} onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
        </div>
      </Card>

      <div className="grid grid-cols-1 xl:grid-cols-12 gap-6">
        <section className="xl:col-span-9 space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {[
              ['Events', stats.total],
              ['Errors', stats.errors],
              ['Slow API', stats.slowRequests],
              ['Slow DB', stats.slowQueries],
              ['Active Verbose', sessions.length],
            ].map(([label, value]) => (
              <Card key={label} variant="high" className="p-4">
                <p className="text-xs text-on-surface-variant uppercase tracking-wide">{label}</p>
                <p className="font-headline text-2xl font-bold mt-1">{value}</p>
              </Card>
            ))}
          </div>

          <Card variant="low" className="overflow-hidden">
            {loading ? (
              <div className="p-10 text-center text-on-surface-variant">Loading logs...</div>
            ) : events.length === 0 ? (
              <EmptyState icon="receipt_long" title="No logs found" description="Adjust the filters or enable a verbose session." />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-surface-container-high text-xs uppercase tracking-wide text-on-surface-variant">
                    <tr>
                      <th className="text-left p-3">Time</th>
                      <th className="text-left p-3">Level</th>
                      <th className="text-left p-3">Component</th>
                      <th className="text-left p-3">Message</th>
                      <th className="text-left p-3">Context</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map((e) => (
                      <tr key={`${e.id}-${e.timestamp}`} onClick={() => setSelected(e)} className="border-t border-outline-variant/15 hover:bg-surface-container-high cursor-pointer">
                        <td className="p-3 whitespace-nowrap text-on-surface-variant">{fmtTime(e.timestamp)}</td>
                        <td className="p-3"><span className={`px-2 py-1 rounded text-xs font-bold uppercase ${levelClass(e.level)}`}>{e.level}</span></td>
                        <td className="p-3 font-medium">{e.component}</td>
                        <td className="p-3 max-w-[420px] truncate">{e.message}</td>
                        <td className="p-3 text-xs text-on-surface-variant">{e.path || e.requestId || (e.deviceId ? `device ${e.deviceId}` : e.eventType)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </section>

        <aside className="xl:col-span-3 space-y-4">
          <Card variant="low" className="p-5 space-y-3">
            <h2 className="font-headline font-bold flex items-center gap-2"><span className="material-symbols-outlined text-primary">tune</span>Verbose</h2>
            <div className="grid grid-cols-2 gap-2">
              <select value={verbose.level} onChange={(e) => setVerbose((v) => ({ ...v, level: e.target.value as 'debug' | 'trace' }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm">
                <option value="debug">Debug</option>
                <option value="trace">Trace</option>
              </select>
              <input type="number" min={1} max={240} value={verbose.durationMinutes} onChange={(e) => setVerbose((v) => ({ ...v, durationMinutes: Number(e.target.value) }))} className="bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
            </div>
            <select value={verbose.component} onChange={(e) => setVerbose((v) => ({ ...v, component: e.target.value }))} className="w-full bg-surface-container-highest rounded-md px-3 py-2 text-sm">
              {COMPONENTS.filter(Boolean).map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
            <input placeholder="Device IDs, comma separated" value={verbose.deviceIds} onChange={(e) => setVerbose((v) => ({ ...v, deviceIds: e.target.value }))} className="w-full bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
            <input placeholder="User IDs, comma separated" value={verbose.userIds} onChange={(e) => setVerbose((v) => ({ ...v, userIds: e.target.value }))} className="w-full bg-surface-container-highest rounded-md px-3 py-2 text-sm" />
            <textarea placeholder="Reason" value={verbose.reason} onChange={(e) => setVerbose((v) => ({ ...v, reason: e.target.value }))} className="w-full bg-surface-container-highest rounded-md px-3 py-2 text-sm min-h-20" />
            <Button className="w-full" icon="play_arrow" onClick={createSession}>Enable</Button>
          </Card>

          <Card variant="low" className="p-5 space-y-3">
            <h2 className="font-headline font-bold">Active Sessions</h2>
            {sessions.length === 0 ? <p className="text-sm text-on-surface-variant">None active.</p> : sessions.map((s) => (
              <div key={s.id} className="border border-outline-variant/20 rounded-md p-3 space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <span className={`px-2 py-1 rounded text-xs font-bold uppercase ${levelClass(s.level)}`}>{s.level}</span>
                  <button onClick={() => stopSession(s.id)} className="material-symbols-outlined text-error text-lg" aria-label="Stop verbose session">stop_circle</button>
                </div>
                <p className="text-xs text-on-surface-variant">{s.components.join(', ') || 'all components'}</p>
                <p className="text-xs">Until {fmtTime(s.expiresAt)}</p>
              </div>
            ))}
          </Card>
        </aside>
      </div>

      {selected && (
        <Card variant="highest" className="fixed right-6 top-24 bottom-6 w-[min(520px,calc(100vw-48px))] z-50 p-5 overflow-auto shadow-xl">
          <div className="flex items-start justify-between gap-4 mb-4">
            <div>
              <p className={`inline-block px-2 py-1 rounded text-xs font-bold uppercase ${levelClass(selected.level)}`}>{selected.level}</p>
              <h2 className="font-headline text-lg font-bold mt-3">{selected.component}</h2>
              <p className="text-xs text-on-surface-variant">{fmtTime(selected.timestamp)}</p>
            </div>
            <button onClick={() => setSelected(null)} className="material-symbols-outlined text-on-surface-variant hover:text-on-surface" aria-label="Close details">close</button>
          </div>
          <p className="mb-4">{selected.message}</p>
          <dl className="grid grid-cols-2 gap-3 text-sm mb-4">
            {Object.entries({
              Event: selected.eventType,
              Request: selected.requestId,
              User: selected.userId,
              Device: selected.deviceId,
              Path: selected.path,
              Status: selected.statusCode,
              Duration: selected.durationMs ? `${selected.durationMs.toFixed(1)} ms` : undefined,
            }).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => (
              <div key={k}>
                <dt className="text-xs text-primary uppercase tracking-wide">{k}</dt>
                <dd className="break-words">{String(v)}</dd>
              </div>
            ))}
          </dl>
          {selected.error && <pre className="bg-error/10 text-error p-3 rounded-md text-xs whitespace-pre-wrap mb-4">{selected.error}</pre>}
          <pre className="bg-surface text-xs p-3 rounded-md overflow-auto whitespace-pre-wrap">{JSON.stringify(selected.attrs || {}, null, 2)}</pre>
        </Card>
      )}
    </div>
  );
}
