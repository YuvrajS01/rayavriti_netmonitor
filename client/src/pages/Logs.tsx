import { useEffect, useMemo, useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '../api/queryKeys';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import SectionHeader from '../components/ui/SectionHeader';
import Button from '../components/ui/Button';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import { createVerboseSession, getLogs, getLogStats, getVerboseSessions, logsExportUrl, stopVerboseSession, type LogEvent, type LogFilters, type LogStats, type VerboseSession } from '../api/logs';
import { useToast } from '../components/ui/useToast';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { StatusPill } from '../components/ui/Status';
import { STATUS_META, type Status } from '../utils/status';

const LEVELS = ['', 'trace', 'debug', 'info', 'warn', 'error'];
const COMPONENTS = ['', 'http', 'db', 'audit', 'collector', 'collector.snmp', 'websocket', 'scheduler'];

const defaultStats: LogStats = { total: 0, byLevel: {}, byComponent: {}, errors: 0, slowRequests: 0, slowQueries: 0 };
const EMPTY_EVENTS: LogEvent[] = [];
const EMPTY_SESSIONS: VerboseSession[] = [];

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

export default function Logs() {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<LogEvent | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [filters, setFilters] = useState({ from: dateTimeLocalHoursAgo(6), to: '', level: '', component: '', q: '', request_id: '', device_id: '' });
  const [verbose, setVerbose] = useState({ level: 'debug' as 'debug' | 'trace', component: 'collector', deviceIds: '', userIds: '', durationMinutes: 30, reason: '' });

  // Free-text inputs would otherwise fire a request per keystroke.
  const debouncedFilters = useDebouncedValue(filters, 300);

  const query = useMemo<LogFilters>(() => ({
    from: toIsoLocal(debouncedFilters.from),
    to: toIsoLocal(debouncedFilters.to),
    level: debouncedFilters.level,
    component: debouncedFilters.component,
    q: debouncedFilters.q,
    request_id: debouncedFilters.request_id,
    device_id: debouncedFilters.device_id,
    limit: 200,
  }), [debouncedFilters]);

  // refetchInterval pauses automatically while the tab is hidden.
  const refetchInterval = autoRefresh ? 10_000 : false;
  const logsQuery = useQuery({
    queryKey: queryKeys.logs.events(query),
    queryFn: ({ signal }) => getLogs(query, { signal }),
    refetchInterval,
    placeholderData: keepPreviousData,
  });
  const statsQuery = useQuery({
    queryKey: queryKeys.logs.stats(query),
    queryFn: ({ signal }) => getLogStats(query, { signal }),
    refetchInterval,
    placeholderData: keepPreviousData,
  });
  const sessionsQuery = useQuery({
    queryKey: queryKeys.logs.verboseSessions(),
    queryFn: ({ signal }) => getVerboseSessions({ signal }),
    refetchInterval,
  });

  const events = logsQuery.data?.data ?? EMPTY_EVENTS;
  const stats = statsQuery.data?.data || defaultStats;
  const sessions = sessionsQuery.data?.data.sessions || EMPTY_SESSIONS;
  // Background (auto-refresh) fetches keep the table on screen; a filter
  // change shows the loader until its first result arrives.
  const loading = logsQuery.isPending || (logsQuery.isFetching && logsQuery.isPlaceholderData);

  const loadFailedAt = logsQuery.errorUpdatedAt;
  useEffect(() => {
    if (loadFailedAt) addToast('Unable to load logs', 'error');
  }, [loadFailedAt, addToast]);

  const refreshLogs = () => queryClient.invalidateQueries({ queryKey: queryKeys.logs.all });

  const createSessionMutation = useMutation({
    mutationFn: (body: Parameters<typeof createVerboseSession>[0]) => createVerboseSession(body),
    onSuccess: () => {
      setVerbose((v) => ({ ...v, reason: '' }));
      addToast('Verbose logging enabled', 'success');
      return refreshLogs();
    },
    onError: () => addToast('Unable to enable verbose logging', 'error'),
  });

  const stopSessionMutation = useMutation({
    mutationFn: (id: number) => stopVerboseSession(id),
    onSuccess: () => {
      addToast('Verbose session stopped', 'success');
      return refreshLogs();
    },
    onError: () => addToast('Unable to stop verbose session', 'error'),
  });

  const createSession = () => {
    if (!verbose.reason.trim()) {
      addToast('Reason is required', 'error');
      return;
    }
    createSessionMutation.mutate({
      level: verbose.level,
      components: verbose.component ? [verbose.component] : [],
      deviceIds: verbose.deviceIds.split(',').map((v) => Number(v.trim())).filter(Boolean),
      userIds: verbose.userIds.split(',').map((v) => v.trim()).filter(Boolean),
      reason: verbose.reason.trim(),
      durationMinutes: verbose.durationMinutes,
    });
  };

  const stopSession = (id: number) => stopSessionMutation.mutate(id);

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Logs"
        subtitle="Search operational events and enable scoped verbose diagnostics."
        action={
          <div className="flex items-center gap-3">
            <Button variant={autoRefresh ? 'primary' : 'secondary'} icon="sync" aria-pressed={autoRefresh} onClick={() => setAutoRefresh((v) => !v)}>
              {autoRefresh ? 'Live' : 'Manual'}
            </Button>
            <a href={logsExportUrl(query)} className="inline-flex min-h-control items-center gap-2 rounded-md border border-line bg-surface-2 px-4 text-sm font-medium text-fg transition-colors hover:border-line-strong hover:bg-surface-3">
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">download</span>
              Export
            </a>
          </div>
        }
      />

      <Card variant="low" className="p-5 overflow-hidden">
        <div className="flex items-center justify-between gap-4 mb-4">
          <div>
            <h2 className="text-sm font-semibold">Log volume by level</h2>
            <p className="text-xs text-fg-muted mt-1">Events in the selected time window</p>
          </div>
          <span className="text-2xl font-semibold tabular-nums">{stats.total.toLocaleString()}</span>
        </div>
        {Object.entries(stats.byLevel).length ? (
          <ul className="flex items-end gap-2 h-24">
            {Object.entries(stats.byLevel).map(([level, value]) => {
              const max = Math.max(...Object.values(stats.byLevel), 1);
              return (
                <li key={level} className="flex-1 h-full flex flex-col justify-end min-w-10">
                  <span className="text-xs text-fg-muted text-center tabular-nums mb-1">{Number(value).toLocaleString()}</span>
                  <i className={`${levelMeta(level).bar} rounded-t-sm transition-[height] duration-[var(--duration-normal)]`} style={{ height: `${Math.max(7, (Number(value) / max) * 100)}%` }} aria-hidden="true" />
                  <span className="text-xs font-mono uppercase text-fg-muted mt-1 text-center">{level}</span>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="py-6 text-center text-sm text-fg-muted">No log activity in this range.</p>
        )}
      </Card>

      <Card variant="low" className="p-4">
        <fieldset>
          <legend className="sr-only">Log filters</legend>
          <div className="grid grid-cols-1 md:grid-cols-7 gap-3">
            <label className={LABEL_CLASS}>From<input type="datetime-local" value={filters.from} onChange={(e) => setFilters((f) => ({ ...f, from: e.target.value }))} className={INPUT_CLASS} /></label>
            <label className={LABEL_CLASS}>To<input type="datetime-local" value={filters.to} onChange={(e) => setFilters((f) => ({ ...f, to: e.target.value }))} className={INPUT_CLASS} /></label>
            <label className={LABEL_CLASS}>Level
              <select value={filters.level} onChange={(e) => setFilters((f) => ({ ...f, level: e.target.value }))} className={INPUT_CLASS}>
                {LEVELS.map((l) => <option key={l} value={l}>{l || 'All levels'}</option>)}
              </select>
            </label>
            <label className={LABEL_CLASS}>Component
              <select value={filters.component} onChange={(e) => setFilters((f) => ({ ...f, component: e.target.value }))} className={INPUT_CLASS}>
                {COMPONENTS.map((c) => <option key={c} value={c}>{c || 'All components'}</option>)}
              </select>
            </label>
            <label className={LABEL_CLASS}>Request ID<input value={filters.request_id} onChange={(e) => setFilters((f) => ({ ...f, request_id: e.target.value }))} className={`${INPUT_CLASS} font-mono`} /></label>
            <label className={LABEL_CLASS}>Device ID<input inputMode="numeric" value={filters.device_id} onChange={(e) => setFilters((f) => ({ ...f, device_id: e.target.value }))} className={INPUT_CLASS} /></label>
            <label className={LABEL_CLASS}>Search<input type="search" value={filters.q} onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value }))} className={INPUT_CLASS} /></label>
          </div>
        </fieldset>
      </Card>

      <div className="grid grid-cols-1 xl:grid-cols-12 gap-6">
        <section className="xl:col-span-9 space-y-4" aria-label="Log events">
          <dl className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {[
              ['Events', stats.total],
              ['Errors', stats.errors],
              ['Slow API', stats.slowRequests],
              ['Slow DB', stats.slowQueries],
              ['Active verbose', sessions.length],
            ].map(([label, value]) => (
              <div key={label} className="rounded-lg border border-line bg-surface-1 p-4">
                <dt className="text-xs text-fg-muted">{label}</dt>
                <dd className={`text-2xl font-semibold mt-1 tabular-nums ${label === 'Errors' && Number(value) > 0 ? 'text-error' : ''}`}>{Number(value).toLocaleString()}</dd>
              </div>
            ))}
          </dl>

          <Card variant="low" className="overflow-hidden">
            {loading ? (
              <div className="p-10 text-center text-fg-muted">Loading logs…</div>
            ) : events.length === 0 ? (
              <EmptyState icon="receipt_long" title="No logs found" description="Adjust the filters or enable a verbose session." />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="bg-surface-2 text-xs text-fg-muted">
                    <tr>
                      <th scope="col" className="text-left font-medium p-3">Time</th>
                      <th scope="col" className="text-left font-medium p-3">Level</th>
                      <th scope="col" className="text-left font-medium p-3">Component</th>
                      <th scope="col" className="text-left font-medium p-3">Message</th>
                      <th scope="col" className="text-left font-medium p-3">Context</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map((e) => (
                      <tr key={`${e.id}-${e.timestamp}`} onClick={() => setSelected(e)} className="border-t border-line hover:bg-surface-2 cursor-pointer">
                        <td className="p-3 whitespace-nowrap font-mono text-xs text-fg-muted">{fmtTime(e.timestamp)}</td>
                        <td className="p-3"><LevelBadge level={e.level} /></td>
                        <td className="p-3 font-medium">{e.component}</td>
                        <td className="p-3 max-w-[420px]">
                          <button type="button" onClick={(ev) => { ev.stopPropagation(); setSelected(e); }} className="block max-w-full truncate text-left hover:underline">
                            {e.message}
                          </button>
                        </td>
                        <td className="p-3 text-xs text-fg-muted font-mono">{e.path || e.requestId || (e.deviceId ? `device ${e.deviceId}` : e.eventType)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </section>

        <aside className="xl:col-span-3 space-y-4">
          <Card variant="low" className="p-5">
            <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); createSession(); }}>
              <h2 className="font-semibold flex items-center gap-2"><span className="material-symbols-outlined text-fg-muted" aria-hidden="true">tune</span>Verbose session</h2>
              <div className="grid grid-cols-2 gap-2">
                <label className={LABEL_CLASS}>Level
                  <select value={verbose.level} onChange={(e) => setVerbose((v) => ({ ...v, level: e.target.value as 'debug' | 'trace' }))} className={INPUT_CLASS}>
                    <option value="debug">Debug</option>
                    <option value="trace">Trace</option>
                  </select>
                </label>
                <label className={LABEL_CLASS}>Minutes
                  <input type="number" min={1} max={240} value={verbose.durationMinutes} onChange={(e) => setVerbose((v) => ({ ...v, durationMinutes: Number(e.target.value) }))} className={INPUT_CLASS} />
                </label>
              </div>
              <label className={LABEL_CLASS}>Component
                <select value={verbose.component} onChange={(e) => setVerbose((v) => ({ ...v, component: e.target.value }))} className={INPUT_CLASS}>
                  {COMPONENTS.filter(Boolean).map((c) => <option key={c} value={c}>{c}</option>)}
                </select>
              </label>
              <label className={LABEL_CLASS}>Device IDs<input placeholder="Comma separated" value={verbose.deviceIds} onChange={(e) => setVerbose((v) => ({ ...v, deviceIds: e.target.value }))} className={INPUT_CLASS} /></label>
              <label className={LABEL_CLASS}>User IDs<input placeholder="Comma separated" value={verbose.userIds} onChange={(e) => setVerbose((v) => ({ ...v, userIds: e.target.value }))} className={INPUT_CLASS} /></label>
              <label className={LABEL_CLASS}>Reason<textarea value={verbose.reason} onChange={(e) => setVerbose((v) => ({ ...v, reason: e.target.value }))} className="mt-1 w-full min-h-20 rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary" /></label>
              <Button type="submit" className="w-full" icon="play_arrow">Enable</Button>
            </form>
          </Card>

          <Card variant="low" className="p-5 space-y-3">
            <h2 className="font-semibold">Active sessions</h2>
            {sessions.length === 0 ? <p className="text-sm text-fg-muted">None active.</p> : sessions.map((s) => (
              <div key={s.id} className="border border-line rounded-md p-3 space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <LevelBadge level={s.level} />
                  <IconButton icon="stop_circle" size="sm" variant="danger" label={`Stop ${s.level} session for ${s.components.join(', ') || 'all components'}`} onClick={() => stopSession(s.id)} />
                </div>
                <p className="text-xs text-fg-muted">{s.components.join(', ') || 'all components'}</p>
                <p className="text-xs">Until <span className="tabular-nums">{fmtTime(s.expiresAt)}</span></p>
              </div>
            ))}
          </Card>
        </aside>
      </div>

      <Dialog
        open={!!selected}
        onClose={() => setSelected(null)}
        placement="drawer"
        title={selected?.component ?? ''}
        description={selected ? fmtTime(selected.timestamp) : undefined}
        headerActions={selected ? <LevelBadge level={selected.level} /> : undefined}
      >
        {selected && (
          <>
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
                  <dt className="text-xs text-fg-muted">{k}</dt>
                  <dd className="break-words font-mono text-xs">{String(v)}</dd>
                </div>
              ))}
            </dl>
            {selected.error && <pre className="mb-4 whitespace-pre-wrap rounded-md border border-line border-l-4 border-l-error bg-inset p-3 font-mono text-xs text-fg">{selected.error}</pre>}
            <pre className="overflow-auto whitespace-pre-wrap rounded-md border border-line bg-inset p-3 font-mono text-xs text-fg-muted">{JSON.stringify(selected.attrs || {}, null, 2)}</pre>
          </>
        )}
      </Dialog>
    </div>
  );
}

const INPUT_CLASS = 'mt-1 w-full h-control rounded-md border border-line bg-inset px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
const LABEL_CLASS = 'block text-xs font-medium text-fg-muted';

/** Log level → colour treatment; levels stay uppercase because they read as codes. */
function levelMeta(level: string): { status: Status | null; bar: string } {
  if (level === 'error' || level === 'fatal') return { status: 'down', bar: STATUS_META.down.bg };
  if (level === 'warn') return { status: 'warning', bar: STATUS_META.warning.bg };
  if (level === 'info') return { status: 'info', bar: STATUS_META.info.bg };
  return { status: null, bar: 'bg-line-strong' };
}

function LevelBadge({ level }: { level: string }) {
  const { status } = levelMeta(level);
  if (!status) {
    return <span className="inline-flex rounded px-2 py-0.5 font-mono text-xs font-semibold uppercase bg-surface-3 text-fg-muted">{level}</span>;
  }
  return <StatusPill status={status} className="font-mono uppercase">{level}</StatusPill>;
}
