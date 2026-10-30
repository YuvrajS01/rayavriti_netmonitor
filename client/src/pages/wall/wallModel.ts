import type { Status } from '../../utils/status';

export const WALL_VIEWS = ['overview', 'alerts', 'topology'] as const;
export type WallView = (typeof WALL_VIEWS)[number];

export const WALL_VIEW_LABELS: Record<WallView, string> = {
  overview: 'Device status',
  alerts: 'Active alerts',
  topology: 'Locations',
};

const MIN_INTERVAL_S = 10;
const DEFAULT_INTERVAL_S = 30;

export interface WallConfig {
  /** Views to cycle through; one entry means no rotation. */
  views: WallView[];
  intervalS: number;
}

/** `?rotate=overview,alerts&interval=30` or `?view=alerts`. Unknown names are dropped. */
export function parseWallParams(params: URLSearchParams): WallConfig {
  const rotate = (params.get('rotate') ?? '')
    .split(',')
    .map((v) => v.trim())
    .filter((v): v is WallView => (WALL_VIEWS as readonly string[]).includes(v));
  const single = params.get('view');
  const views = rotate.length ? [...new Set(rotate)] : (WALL_VIEWS as readonly string[]).includes(single ?? '') ? [single as WallView] : ['overview' as const];
  const raw = Number(params.get('interval'));
  const intervalS = Number.isFinite(raw) && raw > 0 ? Math.max(MIN_INTERVAL_S, Math.round(raw)) : DEFAULT_INTERVAL_S;
  return { views, intervalS };
}

export interface Freshness {
  status: Status;
  label: string;
  ageS: number | null;
}

/**
 * How trustworthy the screen is. Stale when nothing arrived for more than
 * 2× the poll interval (warning) or 4× (down); a dropped socket is at least
 * a warning because updates have stopped even if the last data is recent.
 */
export function dataFreshness({ lastDataAt, now, pollIntervalS, connected }: { lastDataAt: number | null; now: number; pollIntervalS: number; connected: boolean }): Freshness {
  if (lastDataAt == null) return { status: connected ? 'unknown' : 'warning', label: 'No data yet', ageS: null };
  const ageS = Math.max(0, Math.round((now - lastDataAt) / 1000));
  const interval = Math.max(5, pollIntervalS);
  let status: Status = 'up';
  if (ageS > 4 * interval) status = 'down';
  else if (ageS > 2 * interval || !connected) status = 'warning';
  return { status, label: formatAge(ageS), ageS };
}

export function formatAge(ageS: number): string {
  if (ageS < 60) return `${ageS}s old`;
  if (ageS < 3600) return `${Math.floor(ageS / 60)}m ${ageS % 60}s old`;
  return `${Math.floor(ageS / 3600)}h ${Math.floor((ageS % 3600) / 60)}m old`;
}

/** Median of the devices' poll intervals (seconds); 60 when unknown. */
export function typicalInterval(intervals: number[]): number {
  const valid = intervals.filter((n) => Number.isFinite(n) && n > 0).sort((a, b) => a - b);
  if (!valid.length) return 60;
  return valid[Math.floor(valid.length / 2)]!;
}
