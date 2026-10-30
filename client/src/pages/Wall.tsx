import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getDevices, getLatestMetrics } from '../api/devices';
import { getAlerts, ALERT_LIST_LIMIT } from '../api/alerts';
import { queryKeys } from '../api/queryKeys';
import type { Alert, Device, Metric } from '../api/types';
import IconButton from '../components/ui/IconButton';
import { StatusDot, StatusPill } from '../components/ui/Status';
import { useSocketConnected, useSocket } from '../hooks/useSocket';
import { useResourceList } from '../hooks/useResources';
import { applyPreferences } from '../theme/theme';
import { STATUS_META, STATUS_SEVERITY_ORDER, normalizeStatus, severityMeta, statusRank, type Status } from '../utils/status';
import { formatRelative } from '../utils/time';
import { dataFreshness, parseWallParams, typicalInterval, WALL_VIEW_LABELS, type WallView } from './wall/wallModel';
import IconColor from '../assets/brand/Icon-color.svg';

const THEME_KEY = 'nm-wall-theme';
const CHIME_KEY = 'nm-wall-chime';
const FLASH_KEY = 'nm-wall-flash';
const MAX_TILES = 96;
const EMPTY_ALERTS: Alert[] = [];

function readFlag(key: string): boolean {
  try { return window.localStorage.getItem(key) === '1'; } catch { return false; }
}
function writeFlag(key: string, value: boolean | string) {
  try { window.localStorage.setItem(key, typeof value === 'string' ? value : value ? '1' : '0'); } catch { /* per-tab only */ }
}

/** Two short tones via WebAudio; no asset needed. */
function playChime() {
  try {
    const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!Ctx) return;
    const ctx = new Ctx();
    [880, 660].forEach((freq, i) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.frequency.value = freq;
      const start = ctx.currentTime + i * 0.25;
      gain.gain.setValueAtTime(0.0001, start);
      gain.gain.exponentialRampToValueAtTime(0.3, start + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.22);
      osc.connect(gain).connect(ctx.destination);
      osc.start(start);
      osc.stop(start + 0.24);
    });
    setTimeout(() => void ctx.close(), 1000);
  } catch {
    // Autoplay blocked until the operator interacts with the page.
  }
}

interface WallDevice extends Device {
  live: Status;
  metric?: Metric;
}

/**
 * NOC wall display: no app chrome, 1.5× type, worst-first status, a live
 * alert strip, a big clock and an honest data-age indicator. Optional
 * rotation (?rotate=overview,alerts,topology&interval=30), chime and
 * border flash for a *new* critical (both off by default, remembered).
 */
export default function Wall() {
  const [params] = useSearchParams();
  const config = useMemo(() => parseWallParams(params), [params]);
  const connected = useSocketConnected();
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const fromUrl = params.get('theme');
    if (fromUrl === 'light' || fromUrl === 'dark') return fromUrl;
    try { return window.localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark'; } catch { return 'dark'; }
  });
  const [chime, setChime] = useState(() => readFlag(CHIME_KEY));
  const [flashOn, setFlashOn] = useState(() => readFlag(FLASH_KEY));
  const [flashCount, setFlashCount] = useState(0);
  const [paused, setPaused] = useState(false);
  const [viewIndex, setViewIndex] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const [lastEventAt, setLastEventAt] = useState<number | null>(null);
  const view: WallView = config.views[viewIndex % config.views.length] ?? 'overview';

  // 1.5× type and the wall's own theme while mounted; restore on exit.
  useEffect(() => {
    const root = document.documentElement;
    root.dataset.wall = '';
    root.dataset.theme = theme;
    return () => {
      delete root.dataset.wall;
      applyPreferences();
    };
  }, [theme]);

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    if (config.views.length < 2 || paused) return;
    const t = setInterval(() => setViewIndex((i) => (i + 1) % config.views.length), config.intervalS * 1000);
    return () => clearInterval(t);
  }, [config.views.length, config.intervalS, paused]);

  // All three are kept live by the cache bridge; polling is the fallback
  // when the socket is down.
  const devicesQuery = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }), refetchInterval: connected ? false : 30_000 });
  const metricsQuery = useQuery({ queryKey: queryKeys.metrics.latest, queryFn: ({ signal }) => getLatestMetrics({ signal }), refetchInterval: connected ? false : 30_000 });
  const alertsQuery = useQuery({ queryKey: queryKeys.alerts.list('active', ALERT_LIST_LIMIT), queryFn: ({ signal }) => getAlerts('active', ALERT_LIST_LIMIT, { signal }), refetchInterval: connected ? false : 30_000 });
  const locationsQuery = useResourceList('/locations');

  useSocket({ onMetricUpdate: () => setLastEventAt(Date.now()) });

  const devices = useMemo<WallDevice[]>(() => {
    const metrics = new Map((metricsQuery.data?.data ?? []).map((m) => [m.deviceId, m]));
    return (devicesQuery.data?.data ?? [])
      .map((d) => {
        const metric = metrics.get(d.id);
        return { ...d, metric, live: normalizeStatus(metric?.status || d.status) };
      })
      .sort((a, b) => statusRank(a.live) - statusRank(b.live) || a.name.localeCompare(b.name));
  }, [devicesQuery.data, metricsQuery.data]);

  const alerts = alertsQuery.data?.data ?? EMPTY_ALERTS;
  const sortedAlerts = useMemo(
    () => [...alerts].sort((a, b) => (a.severity === 'critical' ? 0 : 1) - (b.severity === 'critical' ? 0 : 1) || new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()),
    [alerts],
  );

  // A critical alert we haven't seen before (after the first load) triggers
  // the chime/flash once.
  const seen = useRef<Set<number> | null>(null);
  useEffect(() => {
    if (!alertsQuery.isSuccess) return;
    if (seen.current === null) {
      seen.current = new Set(alerts.map((a) => a.id));
      return;
    }
    const fresh = alerts.filter((a) => !seen.current!.has(a.id));
    fresh.forEach((a) => seen.current!.add(a.id));
    if (!fresh.some((a) => a.severity === 'critical')) return;
    if (chime) playChime();
    // A new key restarts the three-pulse animation on the overlay.
    if (flashOn) requestAnimationFrame(() => setFlashCount((n) => n + 1));
  }, [alerts, alertsQuery.isSuccess, chime, flashOn]);

  const lastMetricAt = useMemo(() => {
    let max = 0;
    for (const m of metricsQuery.data?.data ?? []) {
      const t = new Date(m.timestamp || m.createdAt).getTime();
      if (Number.isFinite(t) && t > max) max = t;
    }
    return max || null;
  }, [metricsQuery.data]);
  const lastDataAt = Math.max(lastMetricAt ?? 0, lastEventAt ?? 0) || null;
  const freshness = dataFreshness({ lastDataAt, now, pollIntervalS: typicalInterval(devices.map((d) => d.interval)), connected });

  const counts = useMemo(() => {
    const c: Record<Status, number> = { down: 0, warning: 0, unknown: 0, maintenance: 0, info: 0, up: 0 };
    for (const d of devices) c[d.live] += 1;
    return c;
  }, [devices]);

  const toggleTheme = () => setTheme((t) => { const next = t === 'dark' ? 'light' : 'dark'; writeFlag(THEME_KEY, next); return next; });
  const toggleChime = () => setChime((v) => { writeFlag(CHIME_KEY, !v); if (!v) playChime(); return !v; });
  const toggleFlash = () => setFlashOn((v) => { writeFlag(FLASH_KEY, !v); return !v; });
  const fullscreen = () => {
    if (document.fullscreenElement) void document.exitFullscreen?.();
    else void document.documentElement.requestFullscreen?.().catch(() => undefined);
  };

  const clock = new Date(now);
  return (
    <div className="flex min-h-dvh flex-col bg-canvas text-fg">
      {flashCount > 0 && <div key={flashCount} aria-hidden="true" className="wall-flash pointer-events-none fixed inset-0 z-tooltip" />}
      <h1 className="sr-only">NOC wall display</h1>
      {!connected && (
        <div role="alert" className="flex items-center justify-center gap-3 bg-status-warning px-4 py-2 font-semibold text-on-status">
          <span className="material-symbols-outlined" aria-hidden="true">sync_problem</span>
          Live updates disconnected. Reconnecting; the screen refreshes every 30 seconds meanwhile.
        </div>
      )}

      <header className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-line px-6 py-3">
        <img src={IconColor} alt="" className="h-8 w-8" aria-hidden="true" />
        <div className="min-w-0">
          <p className="text-xs text-fg-muted">Rayavriti NetMonitor</p>
          <p className="text-xl font-semibold">{WALL_VIEW_LABELS[view]}</p>
        </div>
        {config.views.length > 1 && (
          <ol className="flex gap-1.5" aria-label="Rotation">
            {config.views.map((v, i) => (
              <li key={v}>
                <button type="button" onClick={() => setViewIndex(i)} aria-current={v === view ? 'step' : undefined} className={`h-2 w-8 rounded-full ${v === view ? 'bg-primary' : 'bg-surface-3'}`}>
                  <span className="sr-only">{WALL_VIEW_LABELS[v]}</span>
                </button>
              </li>
            ))}
          </ol>
        )}

        <div className="ml-auto flex items-center gap-6">
          <div role="status" aria-live="polite" className="flex items-center gap-2" title={lastDataAt ? `Last data ${new Date(lastDataAt).toLocaleTimeString()}` : 'No data received'}>
            <StatusPill status={freshness.status} size="md">Data {freshness.label}</StatusPill>
          </div>
          <time dateTime={clock.toISOString()} className="font-mono text-3xl font-semibold tabular-nums">
            {clock.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          </time>
          <div className="flex items-center gap-1">
            {config.views.length > 1 && <IconButton icon={paused ? 'play_arrow' : 'pause'} label={paused ? 'Resume rotation' : 'Pause rotation'} pressed={paused} onClick={() => setPaused((p) => !p)} />}
            <IconButton icon={chime ? 'notifications_active' : 'notifications_off'} label="Chime on new critical" pressed={chime} onClick={toggleChime} />
            <IconButton icon="flash_on" label="Flash border on new critical" pressed={flashOn} onClick={toggleFlash} />
            <IconButton icon={theme === 'dark' ? 'light_mode' : 'dark_mode'} label={theme === 'dark' ? 'Switch to light' : 'Switch to dark'} onClick={toggleTheme} />
            <IconButton icon="fullscreen" label="Toggle full screen" onClick={fullscreen} />
            <Link to="/" aria-label="Exit wall mode" title="Exit wall mode" className="inline-flex h-10 w-10 items-center justify-center rounded-md text-fg-muted hover:bg-surface-2 hover:text-fg">
              <span className="material-symbols-outlined text-[20px]" aria-hidden="true">close</span>
            </Link>
          </div>
        </div>
      </header>

      <section aria-label="Device status summary" className="grid grid-cols-2 gap-3 px-6 pt-4 md:grid-cols-4">
        {(['down', 'warning', 'unknown', 'up'] as const).map((s) => (
          <div key={s} className={`flex items-center justify-between rounded-lg border-2 bg-surface-1 px-5 py-3 ${counts[s] && s !== 'up' ? STATUS_META[s].border : 'border-line'}`}>
            <span className="flex items-center gap-2 text-base text-fg-muted"><StatusDot status={s} size={14} label="" />{STATUS_META[s].label}</span>
            <span className={`text-3xl font-semibold tabular-nums ${counts[s] && s !== 'up' ? STATUS_META[s].text : 'text-fg'}`}>{counts[s]}</span>
          </div>
        ))}
      </section>

      <main className="min-h-0 flex-1 overflow-hidden px-6 py-4">
        {view === 'overview' && <DeviceGrid devices={devices} loading={devicesQuery.isPending} />}
        {view === 'alerts' && <AlertBoard alerts={sortedAlerts} now={now} />}
        {view === 'topology' && <LocationGrid devices={devices} locations={locationsQuery.data ?? []} />}
      </main>

      <footer aria-label="Newest active alerts" className="flex items-center gap-4 overflow-hidden border-t border-line bg-surface-1 px-6 py-3">
        <span className="shrink-0 text-sm font-semibold text-fg-muted">{alerts.length} active</span>
        <ul className="flex min-w-0 flex-1 gap-6 overflow-hidden">
          {sortedAlerts.length === 0 ? (
            <li className="text-sm text-fg-muted">No active alerts</li>
          ) : (
            sortedAlerts.slice(0, 6).map((a) => (
              <li key={a.id} className="flex shrink-0 items-center gap-2 text-sm">
                <StatusDot status={severityMeta(a.severity)} size={12} />
                <span className="font-medium">{a.deviceName || `Device ${a.deviceId}`}</span>
                <span className="max-w-[24ch] truncate text-fg-muted">{a.message}</span>
                <span className="text-fg-subtle">{formatRelative(a.createdAt, now)}</span>
              </li>
            ))
          )}
        </ul>
      </footer>
    </div>
  );
}

function DeviceGrid({ devices, loading }: { devices: WallDevice[]; loading: boolean }) {
  if (loading) return <p role="status" className="text-fg-muted">Loading devices…</p>;
  if (!devices.length) return <p className="text-fg-muted">No devices are monitored yet.</p>;
  // Problems always fit; healthy devices beyond the budget collapse into a count.
  const problems = devices.filter((d) => d.live !== 'up');
  const shown = problems.length >= MAX_TILES ? problems.slice(0, MAX_TILES) : [...problems, ...devices.filter((d) => d.live === 'up').slice(0, MAX_TILES - problems.length)];
  const hidden = devices.length - shown.length;
  return (
    <ul className="grid gap-2" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(11rem, 1fr))' }} aria-label="Devices, worst first">
      {shown.map((d) => {
        const meta = STATUS_META[d.live];
        return (
          <li key={d.id} className={`rounded-md border-2 bg-surface-1 px-3 py-2 ${d.live === 'up' ? 'border-line' : meta.border}`}>
            <div className="flex items-center gap-2">
              <StatusDot status={d.live} size={12} />
              <span className="truncate font-semibold">{d.name}</span>
            </div>
            <div className="mt-0.5 flex justify-between text-xs text-fg-muted">
              <span className={d.live === 'up' ? '' : meta.text}>{meta.label}</span>
              <span className="font-mono">{d.metric?.responseTime != null ? `${Math.round(d.metric.responseTime)} ms` : '—'}</span>
            </div>
          </li>
        );
      })}
      {hidden > 0 && (
        <li className="flex items-center justify-center rounded-md border-2 border-dashed border-line px-3 py-2 text-sm text-fg-muted">+{hidden} more up</li>
      )}
    </ul>
  );
}

function AlertBoard({ alerts, now }: { alerts: Alert[]; now: number }) {
  if (!alerts.length) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-fg-muted">
        <StatusDot status="up" size={28} label="" />
        <p className="text-2xl">No active alerts</p>
      </div>
    );
  }
  return (
    <ul className="space-y-2" aria-label="Active alerts, critical first">
      {alerts.slice(0, 14).map((a) => {
        const meta = severityMeta(a.severity);
        return (
          <li key={a.id} className={`flex items-center gap-4 rounded-md border border-l-8 border-line bg-surface-1 px-4 py-2.5 ${meta.borderL}`}>
            <StatusPill status={meta} size="md" />
            <span className="w-56 shrink-0 truncate font-semibold">{a.deviceName || `Device ${a.deviceId}`}</span>
            <span className="min-w-0 flex-1 truncate">{a.message}</span>
            {a.assignedToName && <span className="shrink-0 text-sm text-fg-muted">{a.assignedToName}</span>}
            <span className="w-24 shrink-0 text-right text-sm text-fg-muted">{formatRelative(a.createdAt, now)}</span>
          </li>
        );
      })}
      {alerts.length > 14 && <li className="px-4 text-sm text-fg-muted">+{alerts.length - 14} more</li>}
    </ul>
  );
}

interface LocationRow { id: number; name: string; parent: number | null; type: string }

/** Top-level locations (and their children rolled up) with the worst device status. */
function LocationGrid({ devices, locations }: { devices: WallDevice[]; locations: Array<Record<string, unknown>> }) {
  const rows: LocationRow[] = locations.map((l) => ({ id: Number(l.id), name: String(l.name), parent: l.parent_id != null ? Number(l.parent_id) : null, type: typeof l.type === 'string' ? l.type : '' }));
  if (!rows.length) return <p className="text-fg-muted">No locations are configured.</p>;
  const byId = new Map(rows.map((r) => [r.id, r]));
  const rootOf = (id: number): number => {
    let cur = byId.get(id);
    const guard = new Set<number>();
    while (cur?.parent != null && byId.has(cur.parent) && !guard.has(cur.id)) {
      guard.add(cur.id);
      cur = byId.get(cur.parent);
    }
    return cur?.id ?? id;
  };
  // Buildings when there's a single campus root, otherwise the roots.
  const roots = rows.filter((r) => r.parent == null);
  const tierParent = roots.length === 1 ? roots[0]!.id : null;
  const tiles = tierParent != null ? rows.filter((r) => r.parent === tierParent) : roots;
  const tileOf = (locId: number): number | undefined => {
    let cur = byId.get(locId);
    while (cur) {
      if (tiles.some((t) => t.id === cur!.id)) return cur.id;
      cur = cur.parent != null ? byId.get(cur.parent) : undefined;
    }
    return tierParent != null ? undefined : rootOf(locId);
  };
  const groups = new Map<number, WallDevice[]>();
  let unplaced = 0;
  for (const d of devices) {
    const t = d.locationId != null ? tileOf(d.locationId) : undefined;
    if (t == null) { unplaced += 1; continue; }
    groups.set(t, [...(groups.get(t) ?? []), d]);
  }
  const worst = (list: WallDevice[]): Status => list.reduce<Status>((w, d) => (statusRank(d.live) < statusRank(w) ? d.live : w), list.length ? 'up' : 'unknown');
  const sorted = [...tiles].sort((a, b) => statusRank(worst(groups.get(a.id) ?? [])) - statusRank(worst(groups.get(b.id) ?? [])) || a.name.localeCompare(b.name));

  return (
    <div>
      <ul className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(16rem, 1fr))' }} aria-label="Locations, worst first">
        {sorted.map((t) => {
          const list = groups.get(t.id) ?? [];
          const status = worst(list);
          const meta = STATUS_META[status];
          const bad = STATUS_SEVERITY_ORDER.filter((s) => s !== 'up').map((s) => [s, list.filter((d) => d.live === s).length] as const).filter(([, n]) => n > 0);
          return (
            <li key={t.id} className={`rounded-lg border-2 bg-surface-1 p-4 ${status === 'up' || !list.length ? 'border-line' : meta.border}`}>
              <div className="flex items-center gap-2">
                <StatusDot status={status} size={16} />
                <span className="truncate text-lg font-semibold">{t.name}</span>
              </div>
              <p className="mt-1 text-sm text-fg-muted">{list.length} {list.length === 1 ? 'device' : 'devices'}{t.type ? ` · ${t.type}` : ''}</p>
              {bad.length > 0 && (
                <p className="mt-2 flex flex-wrap gap-2">
                  {bad.map(([s, n]) => <StatusPill key={s} status={s}>{n} {STATUS_META[s].label.toLowerCase()}</StatusPill>)}
                </p>
              )}
            </li>
          );
        })}
      </ul>
      {unplaced > 0 && <p className="mt-3 text-sm text-fg-muted">{unplaced} devices have no location.</p>}
    </div>
  );
}
