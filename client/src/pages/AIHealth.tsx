import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AreaChart, Area, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Tooltip, ReferenceLine } from 'recharts';
import { getInsights, getInsightsHistory } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { DeviceHealth, InsightItem, InsightsResponse, HealthHistoryPoint, HealthFactors } from '../api/types';
import { AXIS_TICK_STYLE, GRID_COLOR, SERIES_COLORS, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE } from '../utils/chartConfig';
import { HEALTH_BANDS, STATUS_META, healthBandStatusMeta, healthMeta, severityMeta, statusColorVar } from '../utils/status';
import { useChartAnimation } from '../hooks/useReducedMotion';

import SectionHeader from '../components/ui/SectionHeader';
import RadarChart from '../components/charts/RadarChart';
import RingGauge from '../components/ui/RingGauge';
import Button from '../components/ui/Button';
import ErrorState from '../components/ui/ErrorState';
import EmptyState from '../components/ui/EmptyState';
import KpiCard from '../components/reports/KpiCard';
import { StatusPill, StatusShapeIcon } from '../components/ui/Status';

// ── Helpers ────────────────────────────────────────────────────

const TREND_ICON: Record<string, string> = { improving: 'trending_up', degrading: 'trending_down', stable: 'trending_flat' };
const trendIcon = (trend: string) => TREND_ICON[trend] ?? 'trending_flat';

/** Trend in neutral ink with an arrow: direction is information, not a status. */
function Trend({ trend, delta }: { trend: string; delta?: number }) {
  return (
    <span className="inline-flex items-center gap-1 text-xs text-fg-muted">
      <span className="material-symbols-outlined text-sm" aria-hidden="true">{trendIcon(trend)}</span>
      <span className="capitalize">{trend}</span>
      {delta != null && delta !== 0 && <span className="font-mono">({delta > 0 ? '+' : ''}{Number(delta.toFixed(2))})</span>}
    </span>
  );
}

const FACTOR_LABELS: Record<keyof HealthFactors, string> = {
  availability: 'Availability',
  latency: 'Latency',
  alerts: 'Alerts',
  stability: 'Stability',
  ports: 'Port security',
};

const SELECT_CLASS = 'h-control rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary';

// ── Factor breakdown bars (each factor is itself a 0–100 health score) ──

function FactorBreakdown({ factors }: { factors: HealthFactors }) {
  const keys = Object.keys(factors) as (keyof HealthFactors)[];
  return (
    <div className="space-y-2.5">
      {keys.map((key) => {
        const factor = factors[key];
        const meta = healthMeta(factor.score);
        return (
          <div key={key}>
            <div className="flex justify-between text-xs mb-1">
              <span className="text-fg-muted">{FACTOR_LABELS[key]}</span>
              <span className="inline-flex items-center gap-1.5 text-fg">
                <span className={meta.text}><StatusShapeIcon shape={meta.shape} size={8} label={meta.label} /></span>
                <span className="font-mono">{factor.score.toFixed(0)}/100</span>
              </span>
            </div>
            <div className="h-1.5 bg-surface-3 rounded-full overflow-hidden">
              <div
                className="h-full rounded-full factor-bar-fill"
                style={{ width: `${Math.max(2, factor.score)}%`, background: `var(--color-${meta.token})` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── Health distribution segment bar ────────────────────────────

type Distribution = { critical: number; risk: number; watch: number; healthy: number };

function DistributionBar({ distribution }: { distribution: Distribution }) {
  const total = distribution.critical + distribution.risk + distribution.watch + distribution.healthy;
  if (total === 0) return <p className="text-xs text-fg-muted">No scored devices yet</p>;

  // Worst band first; each band has its own status colour and shape.
  const segments = [...HEALTH_BANDS].reverse()
    .map((band) => ({ band, meta: STATUS_META[band.status], count: distribution[band.band] }))
    .filter((s) => s.count > 0);

  return (
    <div>
      <div className="h-3 rounded-full overflow-hidden flex gap-0.5" role="img" aria-label={segments.map((s) => `${s.band.label} ${s.count}`).join(', ')}>
        {segments.map((seg) => (
          <div
            key={seg.band.band}
            className="h-full"
            style={{ width: `${(seg.count / total) * 100}%`, background: statusColorVar(seg.band.status) }}
          />
        ))}
      </div>
      <ul className="flex flex-wrap gap-x-5 gap-y-1 mt-3">
        {segments.map((seg) => (
          <li key={seg.band.band} className="flex items-center gap-1.5 text-xs">
            <span className={seg.meta.text}><StatusShapeIcon shape={seg.meta.shape} /></span>
            <span className="text-fg-muted">{seg.band.label}</span>
            <span className="font-semibold text-fg">{seg.count}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// ── Device score card ──────────────────────────────────────────

function DeviceScoreCard({ device, deviceInsights }: { device: DeviceHealth; deviceInsights: InsightItem[] }) {
  const [expanded, setExpanded] = useState(false);
  const issues = device.issues ?? [];
  const primaryIssue = issues.find((issue) => issue.severity !== 'info') || issues[0];
  const issuesId = `device-${device.deviceId}-issues`;

  return (
    <article aria-labelledby={`device-${device.deviceId}-name`} className="bg-surface-1 rounded-lg border border-line overflow-hidden">
      <div className="p-5 grid grid-cols-1 lg:grid-cols-[auto_1fr] xl:grid-cols-[auto_1fr_200px_260px] gap-5">
        <div className="flex items-center gap-4">
          <RingGauge value={device.score} size={72} strokeWidth={6} decimals={0} valueClassName="text-lg" />
          <div className="min-w-0">
            <h3 id={`device-${device.deviceId}-name`} className="text-lg font-semibold text-fg truncate">{device.deviceName}</h3>
            <StatusPill status={healthBandStatusMeta(device.label)} className="mt-1" />
            <div className="mt-1.5"><Trend trend={device.trend} delta={device.trendDelta} /></div>
          </div>
        </div>

        <div>
          <dl className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <div className="bg-surface-2 rounded-md p-3">
              <dt className="text-xs text-fg-muted">Availability</dt>
              <dd className="font-semibold text-fg">{device.availabilityPercent}%</dd>
            </div>
            <div className="bg-surface-2 rounded-md p-3">
              <dt className="text-xs text-fg-muted">Avg response</dt>
              <dd className="font-semibold text-fg">{device.avgResponseMs} ms</dd>
            </div>
            <div className="bg-surface-2 rounded-md p-3">
              <dt className="text-xs text-fg-muted">Alerts</dt>
              <dd className="font-semibold text-fg">{device.activeAlerts}</dd>
            </div>
            <div className="bg-surface-2 rounded-md p-3">
              <dt className="text-xs text-fg-muted">Open ports</dt>
              <dd className="font-semibold text-fg">{device.openPorts}</dd>
            </div>
          </dl>
          {device.factors && <FactorBreakdown factors={device.factors} />}
        </div>

        {device.factors && (
          <div className="rounded-md border border-line bg-surface-2 p-2 grid place-items-center">
            <RadarChart
              axes={['Avail.', 'Latency', 'Alerts', 'Stable', 'Ports']}
              series={[{ label: device.deviceName, values: [device.factors.availability.score, device.factors.latency.score, device.factors.alerts.score, device.factors.stability.score, device.factors.ports.score] }]}
              size={180}
            />
          </div>
        )}

        <div className="bg-surface-2 rounded-md p-4 border border-line">
          <p className="text-xs text-fg-muted mb-2">Primary issue</p>
          <p className="text-sm font-semibold text-fg leading-snug">{primaryIssue?.message || 'No issue detected'}</p>
          <div className="mt-4 pt-4 border-t border-line">
            <p className="text-xs text-fg-muted mb-2">Related insights</p>
            {deviceInsights.length === 0 ? (
              <p className="text-xs text-fg-muted">No extra insights</p>
            ) : (
              <ul className="space-y-2">
                {deviceInsights.slice(0, 2).map((item, index) => (
                  <li key={`${item.type}-${index}`} className="text-xs text-fg-muted leading-relaxed">{item.message}</li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>

      {issues.length > 1 && (
        <div className="border-t border-line">
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
            aria-controls={issuesId}
            className="w-full px-5 py-2.5 flex items-center justify-between text-xs text-fg-muted hover:text-fg transition-colors"
          >
            <span className="font-semibold">{expanded ? 'Hide' : 'Show'} all {issues.length} issues</span>
            <span className="material-symbols-outlined text-sm" aria-hidden="true">{expanded ? 'expand_less' : 'expand_more'}</span>
          </button>
          <div id={issuesId} className="animate-slide-down" data-open={expanded} inert={!expanded || undefined}>
            <ul className="space-y-2 px-5 pb-4">
              {issues.map((issue, index) => {
                const meta = severityMeta(issue.severity);
                return (
                  <li key={`${issue.type}-${index}`} className={`rounded-md border border-line border-l-4 ${meta.borderL} bg-surface-2 px-3 py-2 flex items-start justify-between gap-3`}>
                    <p className="text-xs leading-relaxed text-fg">{issue.message}</p>
                    <StatusPill status={meta} />
                  </li>
                );
              })}
            </ul>
          </div>
        </div>
      )}
    </article>
  );
}

// ── Main Page ──────────────────────────────────────────────────

const EMPTY_HISTORY: HealthHistoryPoint[] = [];
const TIMELINE_COLOR = SERIES_COLORS[0];

export default function AIHealth() {
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [sortBy, setSortBy] = useState('score-asc');
  const anim = useChartAnimation();

  const insightsQuery = useQuery({
    queryKey: queryKeys.insights.current(),
    queryFn: ({ signal }) => getInsights({ signal }),
  });
  // History is optional; a failure just leaves the timeline empty.
  const historyQuery = useQuery({
    queryKey: queryKeys.insights.history(12),
    queryFn: ({ signal }) => getInsightsHistory(12, { signal }),
  });
  const data: InsightsResponse | null = insightsQuery.data?.data ?? null;
  const history: HealthHistoryPoint[] = historyQuery.data?.data?.points || EMPTY_HISTORY;
  const loading = insightsQuery.isFetching || historyQuery.isFetching;
  const error = insightsQuery.isError ? 'Failed to load health data. The API may be unavailable.' : null;
  const load = () => { void insightsQuery.refetch(); void historyQuery.refetch(); };

  const devices = useMemo(() => data?.health || [], [data]);

  const filtered = useMemo(() => {
    let list = devices.filter((device) => {
      const matchesFilter = filter === 'all' || device.label === filter;
      const matchesQuery = !query || device.deviceName.toLowerCase().includes(query.toLowerCase());
      return matchesFilter && matchesQuery;
    });

    // Sort
    switch (sortBy) {
      case 'score-asc':
        list = [...list].sort((a, b) => a.score - b.score);
        break;
      case 'score-desc':
        list = [...list].sort((a, b) => b.score - a.score);
        break;
      case 'trend':
        list = [...list].sort((a, b) => a.trendDelta - b.trendDelta);
        break;
      case 'name':
        list = [...list].sort((a, b) => a.deviceName.localeCompare(b.deviceName));
        break;
    }

    return list;
  }, [devices, filter, query, sortBy]);

  const networkScore = data?.networkScore ?? 0;
  const distribution = data?.healthDistribution ?? { critical: 0, risk: 0, watch: 0, healthy: 0 };
  const topRisks = data?.topRisks ?? [];
  const totalIssues = devices.reduce((sum, item) => sum + (item.issues ?? []).filter((issue) => issue.type !== 'clear').length, 0);

  // Prepare timeline data for Recharts
  const timelineData = useMemo(() => {
    return history
      .filter((p) => p.score !== null)
      .map((p) => ({
        time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        score: p.score,
      }));
  }, [history]);

  // Find network trend from first vs last history points
  const networkTrend = useMemo(() => {
    const valid = history.filter((p) => p.score !== null);
    if (valid.length < 2) return { trend: 'stable' as const, delta: 0 };
    const first = valid[0]?.score ?? 0;
    const last = valid[valid.length - 1]?.score ?? 0;
    const delta = last - first;
    return {
      trend: delta >= 3 ? 'improving' as const : delta <= -3 ? 'degrading' as const : 'stable' as const,
      delta,
    };
  }, [history]);

  return (
    <div>
      <SectionHeader
        title="AI health score"
        subtitle="Weighted device risk scoring from availability, latency, alerts, stability and port changes, with trend analysis."
        action={<Button variant="secondary" icon="refresh" onClick={load} disabled={loading}>Refresh</Button>}
      />

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        <section aria-label="Network health" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col items-center justify-center gap-3">
          <RingGauge value={networkScore} size={160} strokeWidth={12} label="Network health" />
          <Trend trend={networkTrend.trend} delta={networkTrend.delta} />
          <p className="text-xs text-fg-muted">
            {devices.length} device{devices.length !== 1 ? 's' : ''} monitored
          </p>
        </section>

        <section aria-labelledby="health-distribution-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
          <h3 id="health-distribution-heading" className="text-sm font-semibold text-fg mb-4">Health distribution</h3>
          <DistributionBar distribution={distribution} />

          {topRisks.length > 0 && (
            <div className="mt-6 pt-4 border-t border-line">
              <h4 className="text-xs text-fg-muted mb-3 font-semibold">Top risks</h4>
              <ul className="space-y-3">
                {topRisks.map((risk) => (
                  <li key={risk.deviceId} className="flex items-center gap-3">
                    <span className="w-10 shrink-0 text-right font-mono text-sm font-semibold text-fg">{risk.score.toFixed(0)}</span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-semibold text-fg truncate">{risk.deviceName}</p>
                      <p className="text-xs text-fg-muted truncate">{risk.primaryIssue}</p>
                    </div>
                    <StatusPill status={healthMeta(risk.score)} />
                    <span className="material-symbols-outlined text-sm text-fg-muted" role="img" aria-label={`Trend ${risk.trend}`}>
                      {trendIcon(risk.trend)}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </section>

        <section aria-labelledby="health-timeline-heading" className="bg-surface-1 rounded-lg p-5 border border-line">
          <h3 id="health-timeline-heading" className="text-sm font-semibold text-fg mb-4">Network score, last 12 hours</h3>
          {timelineData.length < 2 ? (
            <div className="flex items-center justify-center h-40 text-xs text-fg-muted">
              Not enough history data yet
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={timelineData} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="healthGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor={TIMELINE_COLOR} stopOpacity={0.25} />
                    <stop offset="95%" stopColor={TIMELINE_COLOR} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} width={36} />
                {/* Band boundaries as quiet references (watch / risk / critical). */}
                {HEALTH_BANDS.filter((b) => Number.isFinite(b.min)).map((b) => (
                  <ReferenceLine key={b.band} y={b.min} stroke="var(--color-line-strong)" strokeDasharray="2 4" />
                ))}
                <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)} (${healthMeta(Number(v ?? 0)).label})`, 'Score']} />
                <Area
                  type="monotone"
                  dataKey="score"
                  stroke={TIMELINE_COLOR}
                  strokeWidth={2}
                  fill="url(#healthGradient)"
                  dot={false}
                  activeDot={{ r: 4, fill: TIMELINE_COLOR }}
                  {...anim}
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </section>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4 mb-6">
        <KpiCard icon="monitoring" label="Average score" value={networkScore.toFixed(0)} status={healthMeta(networkScore)} />
        <KpiCard icon="dangerous" label="Critical devices" value={distribution.critical} status={distribution.critical > 0 ? healthBandStatusMeta('critical') : undefined} />
        <KpiCard icon="visibility" label="Watch list" value={distribution.watch + distribution.risk} sub={`${distribution.risk} at risk · ${distribution.watch} watch`} />
        <KpiCard icon="report" label="Open issues" value={totalIssues} />
      </div>

      {error && !loading && <div className="mb-6"><ErrorState message={error} onRetry={load} /></div>}

      <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6 flex flex-col lg:flex-row gap-3">
        <label className="relative flex-1">
          <span className="sr-only">Search devices</span>
          <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search devices"
            className={`${SELECT_CLASS} w-full pl-9 placeholder:text-fg-subtle`}
          />
        </label>
        <select aria-label="Filter by health band" value={filter} onChange={(event) => setFilter(event.target.value)} className={SELECT_CLASS}>
          <option value="all">All bands</option>
          {[...HEALTH_BANDS].reverse().map((b) => <option key={b.band} value={b.band}>{b.label}</option>)}
        </select>
        <select aria-label="Sort devices" value={sortBy} onChange={(event) => setSortBy(event.target.value)} className={SELECT_CLASS}>
          <option value="score-asc">Score, worst first</option>
          <option value="score-desc">Score, best first</option>
          <option value="trend">Trend, most declining</option>
          <option value="name">Name A–Z</option>
        </select>
      </div>

      {loading ? (
        <div role="status" className="bg-surface-1 rounded-lg border border-line py-16 text-center text-sm text-fg-muted">Calculating health scores…</div>
      ) : error && !data ? null : filtered.length === 0 ? (
        <div className="bg-surface-1 rounded-lg border border-line">
          <EmptyState icon="filter_alt_off" title="No devices match this view" description="Change the band filter or search." />
        </div>
      ) : (
        <div className="space-y-4">
          {filtered.map((device) => (
            <DeviceScoreCard
              key={device.deviceId}
              device={device}
              deviceInsights={(data?.insights || []).filter((item) => item.deviceId === device.deviceId)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
