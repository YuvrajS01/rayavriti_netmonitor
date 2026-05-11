import { useCallback, useEffect, useMemo, useState } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { getInsights, getInsightsHistory } from '../api/client';
import type { DeviceHealth, InsightItem, InsightsResponse, HealthHistoryPoint, HealthFactors } from '../api/types';

// ── Color helpers ──────────────────────────────────────────────

function scoreColor(score: number) {
  if (score < 40) return 'text-error';
  if (score < 70) return 'text-amber-400';
  return 'text-primary';
}

function scoreBg(score: number) {
  if (score < 40) return '#ff4444';
  if (score < 70) return '#f59e0b';
  return '#d9fd3a';
}

function scoreGlow(label: string) {
  if (label === 'critical') return 'glow-critical';
  if (label === 'risk') return 'glow-risk';
  if (label === 'watch') return 'glow-watch';
  return 'glow-healthy';
}

function severityStyle(severity: string) {
  if (severity === 'critical') return 'text-error bg-error/10 border-error/25';
  if (severity === 'warning') return 'text-amber-400 bg-amber-400/10 border-amber-400/25';
  return 'text-primary bg-primary/10 border-primary/20';
}

function trendIcon(trend: string) {
  if (trend === 'improving') return 'trending_up';
  if (trend === 'degrading') return 'trending_down';
  return 'trending_flat';
}

function trendColor(trend: string) {
  if (trend === 'improving') return 'text-primary';
  if (trend === 'degrading') return 'text-error';
  return 'text-on-surface-variant';
}

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

const FACTOR_LABELS: Record<keyof HealthFactors, string> = {
  availability: 'Availability',
  latency: 'Latency',
  alerts: 'Alerts',
  stability: 'Stability',
  ports: 'Port Security',
};

const FACTOR_COLORS: Record<keyof HealthFactors, string> = {
  availability: '#d9fd3a',
  latency: '#6ee7f7',
  alerts: '#ff7351',
  stability: '#c084fc',
  ports: '#4ade80',
};

// ── SVG Radial Gauge ───────────────────────────────────────────

function RadialGauge({ score, size = 140, strokeWidth = 10, label }: { score: number; size?: number; strokeWidth?: number; label?: string }) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (score / 100) * circumference;
  const center = size / 2;

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="transform -rotate-90">
        {/* Background ring */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="#26261d"
          strokeWidth={strokeWidth}
        />
        {/* Score ring */}
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={scoreBg(score)}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          className="gauge-ring"
          style={{
            '--gauge-circumference': circumference,
            '--gauge-offset': offset,
            strokeDashoffset: offset,
            filter: `drop-shadow(0 0 8px ${scoreBg(score)}40)`,
          } as React.CSSProperties}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={`font-headline text-3xl font-black ${scoreColor(score)}`}>{score}</span>
        {label && <span className="text-[10px] uppercase tracking-widest text-on-surface-variant mt-0.5">{label}</span>}
      </div>
    </div>
  );
}

// ── Small Gauge for device cards ───────────────────────────────

function MiniGauge({ score, size = 72, strokeWidth = 6 }: { score: number; size?: number; strokeWidth?: number }) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (score / 100) * circumference;
  const center = size / 2;

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="transform -rotate-90">
        <circle cx={center} cy={center} r={radius} fill="none" stroke="#26261d" strokeWidth={strokeWidth} />
        <circle
          cx={center} cy={center} r={radius}
          fill="none" stroke={scoreBg(score)} strokeWidth={strokeWidth} strokeLinecap="round"
          strokeDasharray={circumference}
          className="gauge-ring"
          style={{ '--gauge-circumference': circumference, '--gauge-offset': offset, strokeDashoffset: offset } as React.CSSProperties}
        />
      </svg>
      <span className={`absolute font-headline text-xl font-black ${scoreColor(score)}`}>{score}</span>
    </div>
  );
}

// ── Factor Breakdown Bars ──────────────────────────────────────

function FactorBreakdown({ factors }: { factors: HealthFactors }) {
  const keys = Object.keys(factors) as (keyof HealthFactors)[];
  return (
    <div className="space-y-2.5">
      {keys.map((key) => {
        const factor = factors[key];
        return (
          <div key={key}>
            <div className="flex justify-between text-[10px] uppercase tracking-widest mb-1">
              <span className="text-on-surface-variant">{FACTOR_LABELS[key]}</span>
              <span style={{ color: FACTOR_COLORS[key] }}>{factor.score}/100</span>
            </div>
            <div className="h-1.5 bg-surface-container-highest rounded-full overflow-hidden">
              <div
                className="h-full rounded-full factor-bar-fill"
                style={{ width: `${Math.max(2, factor.score)}%`, background: FACTOR_COLORS[key] }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── Health Distribution Segment Bar ────────────────────────────

function DistributionBar({ distribution }: { distribution: { critical: number; risk: number; watch: number; healthy: number } }) {
  const total = distribution.critical + distribution.risk + distribution.watch + distribution.healthy;
  if (total === 0) return null;

  const segments = [
    { key: 'critical', count: distribution.critical, color: '#ff4444', label: 'Critical' },
    { key: 'risk', count: distribution.risk, color: '#ff7351', label: 'Risk' },
    { key: 'watch', count: distribution.watch, color: '#f59e0b', label: 'Watch' },
    { key: 'healthy', count: distribution.healthy, color: '#d9fd3a', label: 'Healthy' },
  ].filter((s) => s.count > 0);

  return (
    <div>
      <div className="h-3 rounded-full overflow-hidden flex">
        {segments.map((seg) => (
          <div
            key={seg.key}
            className="h-full transition-all duration-700 first:rounded-l-full last:rounded-r-full"
            style={{ width: `${(seg.count / total) * 100}%`, background: seg.color }}
            title={`${seg.label}: ${seg.count}`}
          />
        ))}
      </div>
      <div className="flex flex-wrap gap-x-5 gap-y-1 mt-3">
        {segments.map((seg) => (
          <div key={seg.key} className="flex items-center gap-1.5 text-xs">
            <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ background: seg.color }} />
            <span className="text-on-surface-variant">{seg.label}</span>
            <span className="font-bold text-on-surface">{seg.count}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Device Score Card ──────────────────────────────────────────

function DeviceScoreCard({ device, deviceInsights }: { device: DeviceHealth; deviceInsights: InsightItem[] }) {
  const [expanded, setExpanded] = useState(false);
  const primaryIssue = device.issues.find((issue) => issue.severity !== 'info') || device.issues[0];

  return (
    <div className={`bg-surface-container-high rounded-xl border border-outline-variant/20 overflow-hidden transition-all hover:border-outline-variant/40 ${scoreGlow(device.label)}`}>
      <div className="p-5 grid grid-cols-1 xl:grid-cols-[auto_1fr_280px] gap-5">
        {/* Left: Gauge + Name + Trend */}
        <div className="flex items-center gap-4">
          <MiniGauge score={device.score} />
          <div className="min-w-0">
            <h3 className="font-headline text-lg font-bold text-on-surface truncate">{device.deviceName}</h3>
            <p className={`text-[10px] uppercase tracking-widest font-bold ${scoreColor(device.score)}`}>{device.label}</p>
            <div className={`flex items-center gap-1 mt-1.5 ${trendColor(device.trend)}`}>
              <span className={`material-symbols-outlined text-sm ${device.trend === 'degrading' ? 'trend-pulse' : ''}`}>
                {trendIcon(device.trend)}
              </span>
              <span className="text-[10px] uppercase tracking-widest font-bold">
                {device.trend}{device.trendDelta !== 0 ? ` (${device.trendDelta > 0 ? '+' : ''}${device.trendDelta})` : ''}
              </span>
            </div>
          </div>
        </div>

        {/* Center: Stats + Factor breakdown */}
        <div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
            <div className="bg-surface-container-low rounded-lg p-3">
              <p className="text-[10px] uppercase tracking-widest text-on-surface-variant">Availability</p>
              <p className="font-bold text-on-surface">{device.availabilityPercent}%</p>
            </div>
            <div className="bg-surface-container-low rounded-lg p-3">
              <p className="text-[10px] uppercase tracking-widest text-on-surface-variant">Avg Response</p>
              <p className="font-bold text-on-surface">{device.avgResponseMs}ms</p>
            </div>
            <div className="bg-surface-container-low rounded-lg p-3">
              <p className="text-[10px] uppercase tracking-widest text-on-surface-variant">Alerts</p>
              <p className="font-bold text-on-surface">{device.activeAlerts}</p>
            </div>
            <div className="bg-surface-container-low rounded-lg p-3">
              <p className="text-[10px] uppercase tracking-widest text-on-surface-variant">Open Ports</p>
              <p className="font-bold text-on-surface">{device.openPorts}</p>
            </div>
          </div>

          {device.factors && <FactorBreakdown factors={device.factors} />}
        </div>

        {/* Right: Primary Issue + Insights */}
        <div className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/15">
          <p className="text-[10px] uppercase tracking-widest text-on-surface-variant mb-2">Primary Issue</p>
          <p className="text-sm font-bold text-on-surface leading-snug">{primaryIssue?.message || 'No issue detected'}</p>
          <div className="mt-4 pt-4 border-t border-outline-variant/15">
            <p className="text-[10px] uppercase tracking-widest text-on-surface-variant mb-2">Related Insights</p>
            {deviceInsights.length === 0 ? (
              <p className="text-xs text-on-surface-variant">No extra insights</p>
            ) : (
              <div className="space-y-2">
                {deviceInsights.slice(0, 2).map((item, index) => (
                  <p key={`${item.type}-${index}`} className="text-xs text-on-surface-variant leading-relaxed">{item.message}</p>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Expandable Issues Accordion */}
      {device.issues.length > 1 && (
        <div className="border-t border-outline-variant/15">
          <button
            onClick={() => setExpanded(!expanded)}
            className="w-full px-5 py-2.5 flex items-center justify-between text-xs text-on-surface-variant hover:text-on-surface transition-colors"
          >
            <span className="uppercase tracking-widest font-bold">
              {expanded ? 'Hide' : 'Show'} all {device.issues.length} issues
            </span>
            <span className="material-symbols-outlined text-sm transition-transform" style={{ transform: expanded ? 'rotate(180deg)' : '' }}>
              expand_more
            </span>
          </button>
          {expanded && (
            <div className="px-5 pb-4 animate-slide-down">
              <div className="space-y-2">
                {device.issues.map((issue, index) => (
                  <div key={`${issue.type}-${index}`} className={`rounded-lg border px-3 py-2 flex items-start gap-2 ${severityStyle(issue.severity)}`}>
                    <span className="material-symbols-outlined text-base mt-0.5">{issue.severity === 'critical' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}</span>
                    <p className="text-xs leading-relaxed">{issue.message}</p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ── Main Page ──────────────────────────────────────────────────

export default function AIHealth() {
  const [data, setData] = useState<InsightsResponse | null>(null);
  const [history, setHistory] = useState<HealthHistoryPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [sortBy, setSortBy] = useState('score-asc');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [insightsRes, historyRes] = await Promise.allSettled([
        getInsights(),
        getInsightsHistory(12),
      ]);
      if (insightsRes.status === 'fulfilled') setData(insightsRes.value.data);
      if (historyRes.status === 'fulfilled') setHistory(historyRes.value.data?.points || []);
    } catch {
      // API unavailable — page will show empty state
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const devices = data?.health || [];

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
  const totalIssues = devices.reduce((sum, item) => sum + item.issues.filter((issue) => issue.type !== 'clear').length, 0);

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
    const first = valid[0].score!;
    const last = valid[valid.length - 1].score!;
    const delta = last - first;
    return {
      trend: delta >= 3 ? 'improving' as const : delta <= -3 ? 'degrading' as const : 'stable' as const,
      delta,
    };
  }, [history]);

  return (
    <div>
      {/* Header */}
      <header className="mb-10 flex flex-col xl:flex-row xl:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">AI Health Score</h1>
          <p className="text-on-surface-variant font-body max-w-2xl">Weighted device risk scoring from availability, latency, alerts, stability, and port changes with trend analysis.</p>
        </div>
        <button onClick={load} className="bg-primary text-on-primary font-bold py-3 px-5 rounded-lg tracking-widest uppercase hover:brightness-110 active:scale-95 transition-all text-xs flex items-center justify-center gap-2">
          <span className="material-symbols-outlined text-base">refresh</span>
          Refresh
        </button>
      </header>

      {/* Hero: Network Score + Distribution + Timeline */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-8">
        {/* Network Score Gauge */}
        <div className={`bg-surface-container-high rounded-xl p-6 border border-outline-variant/20 flex flex-col items-center justify-center ${scoreGlow(networkScore >= 85 ? 'healthy' : networkScore >= 65 ? 'watch' : networkScore >= 40 ? 'risk' : 'critical')}`}>
          <RadialGauge score={networkScore} size={160} strokeWidth={12} label="Network" />
          <div className={`flex items-center gap-1.5 mt-4 ${trendColor(networkTrend.trend)}`}>
            <span className={`material-symbols-outlined text-lg ${networkTrend.trend === 'degrading' ? 'trend-pulse' : ''}`}>
              {trendIcon(networkTrend.trend)}
            </span>
            <span className="text-xs uppercase tracking-widest font-bold">
              {networkTrend.trend}
              {networkTrend.delta !== 0 && ` (${networkTrend.delta > 0 ? '+' : ''}${networkTrend.delta})`}
            </span>
          </div>
          <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mt-2">
            {devices.length} device{devices.length !== 1 ? 's' : ''} monitored
          </p>
        </div>

        {/* Distribution + Top Risks */}
        <div className="bg-surface-container-high rounded-xl p-6 border border-outline-variant/20 flex flex-col">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4">Health Distribution</h3>
          <DistributionBar distribution={distribution} />

          {topRisks.length > 0 && (
            <div className="mt-6 pt-4 border-t border-outline-variant/15">
              <h4 className="text-[10px] uppercase tracking-widest text-on-surface-variant mb-3 font-bold">Top Risks</h4>
              <div className="space-y-2.5">
                {topRisks.map((risk) => (
                  <div key={risk.deviceId} className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-full bg-surface-container-low grid place-items-center">
                      <span className={`font-headline text-sm font-black ${scoreColor(risk.score)}`}>{risk.score}</span>
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-bold text-on-surface truncate">{risk.deviceName}</p>
                      <p className="text-[10px] text-on-surface-variant truncate">{risk.primaryIssue}</p>
                    </div>
                    <span className={`material-symbols-outlined text-sm ${trendColor(risk.trend)}`}>
                      {trendIcon(risk.trend)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Health Timeline */}
        <div className="bg-surface-container-high rounded-xl p-6 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4">12-Hour Timeline</h3>
          {timelineData.length < 2 ? (
            <div className="flex items-center justify-center h-40 text-xs text-on-surface-variant">
              Not enough history data yet
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={timelineData} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="healthGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#d9fd3a" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#d9fd3a" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis
                  dataKey="time"
                  tick={{ fill: '#8a8a78', fontSize: 9 }}
                  tickLine={false}
                  axisLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  domain={[0, 100]}
                  tick={{ fill: '#8a8a78', fontSize: 9 }}
                  tickLine={false}
                  axisLine={false}
                  width={30}
                />
                <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [`${Number(v ?? 0)}`, 'Score']} />
                <Area
                  type="monotone"
                  dataKey="score"
                  stroke="#d9fd3a"
                  strokeWidth={2}
                  fill="url(#healthGradient)"
                  dot={false}
                  activeDot={{ r: 4, fill: '#d9fd3a' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-5 mb-6">
        <div className="bg-surface-container-low rounded-xl p-5 border border-outline-variant/20">
          <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-1">Average Score</p>
          <p className={`font-headline text-3xl font-black ${scoreColor(networkScore)}`}>{networkScore}</p>
        </div>
        <div className="bg-surface-container-low rounded-xl p-5 border border-outline-variant/20">
          <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-1">Critical Devices</p>
          <p className="font-headline text-3xl font-black text-error">{distribution.critical}</p>
        </div>
        <div className="bg-surface-container-low rounded-xl p-5 border border-outline-variant/20">
          <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-1">Watch List</p>
          <p className="font-headline text-3xl font-black text-amber-400">{distribution.watch + distribution.risk}</p>
        </div>
        <div className="bg-surface-container-low rounded-xl p-5 border border-outline-variant/20">
          <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-1">Open Issues</p>
          <p className="font-headline text-3xl font-black text-primary">{totalIssues}</p>
        </div>
      </div>

      {/* Filters + Sort */}
      <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 mb-6 flex flex-col lg:flex-row gap-3">
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search devices..."
          className="flex-1 bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
        />
        <select value={filter} onChange={(event) => setFilter(event.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All scores</option>
          <option value="critical">Critical</option>
          <option value="risk">Risk</option>
          <option value="watch">Watch</option>
          <option value="healthy">Healthy</option>
        </select>
        <select value={sortBy} onChange={(event) => setSortBy(event.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="score-asc">Score ↑ (worst first)</option>
          <option value="score-desc">Score ↓ (best first)</option>
          <option value="trend">Trend (most declining)</option>
          <option value="name">Name A–Z</option>
        </select>
      </div>

      {/* Device Cards */}
      {loading ? (
        <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 py-16 text-center text-on-surface-variant animate-pulse">Calculating health scores...</div>
      ) : filtered.length === 0 ? (
        <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 py-16 text-center text-on-surface-variant">No devices match this view</div>
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
