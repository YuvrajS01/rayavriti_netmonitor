import { useCallback, useEffect, useMemo, useState } from 'react';
import { getInsights } from '../api/client';
import type { DeviceHealth, InsightItem, InsightsResponse } from '../api/types';

function scoreColor(score: number) {
  if (score < 40) return 'text-error';
  if (score < 70) return 'text-amber-400';
  return 'text-primary';
}

function scoreBg(score: number) {
  if (score < 40) return '#ff7351';
  if (score < 70) return '#f59e0b';
  return '#d9fd3a';
}

function severityStyle(severity: string) {
  if (severity === 'critical') return 'text-error bg-error/10 border-error/25';
  if (severity === 'warning') return 'text-amber-400 bg-amber-400/10 border-amber-400/25';
  return 'text-primary bg-primary/10 border-primary/20';
}

function SummaryTile({ label, value, tone = 'text-primary' }: { label: string; value: string | number; tone?: string }) {
  return (
    <div className="bg-surface-container-low rounded-xl p-5 border border-outline-variant/20">
      <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-1">{label}</p>
      <p className={`font-headline text-3xl font-black ${tone}`}>{value}</p>
    </div>
  );
}

function DeviceScoreRow({ device, deviceInsights }: { device: DeviceHealth; deviceInsights: InsightItem[] }) {
  const primaryIssue = device.issues.find((issue) => issue.severity !== 'info') || device.issues[0];

  return (
    <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 overflow-hidden">
      <div className="p-5 grid grid-cols-1 xl:grid-cols-[220px_1fr_220px] gap-5">
        <div>
          <div className="flex items-center gap-3">
            <div className="relative h-20 w-20 rounded-full bg-surface-container-low border border-outline-variant/20 grid place-items-center">
              <span className={`font-headline text-3xl font-black ${scoreColor(device.score)}`}>{device.score}</span>
              <div className="absolute inset-0 rounded-full border-4 border-transparent" style={{ borderTopColor: scoreBg(device.score), transform: `rotate(${Math.max(8, device.score * 3.6)}deg)` }} />
            </div>
            <div className="min-w-0">
              <h3 className="font-headline text-xl font-bold text-on-surface truncate">{device.deviceName}</h3>
              <p className={`text-[10px] uppercase tracking-widest font-bold ${scoreColor(device.score)}`}>{device.label}</p>
            </div>
          </div>
          <div className="mt-4 h-2 bg-surface-container-highest rounded">
            <div className="h-2 rounded transition-all duration-500" style={{ width: `${Math.max(3, device.score)}%`, background: scoreBg(device.score) }} />
          </div>
        </div>

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

          <div className="space-y-2">
            {device.issues.map((issue, index) => (
              <div key={`${issue.type}-${index}`} className={`rounded-lg border px-3 py-2 flex items-start gap-2 ${severityStyle(issue.severity)}`}>
                <span className="material-symbols-outlined text-base mt-0.5">{issue.severity === 'critical' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}</span>
                <p className="text-xs leading-relaxed">{issue.message}</p>
              </div>
            ))}
          </div>
        </div>

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
    </div>
  );
}

export default function AIHealth() {
  const [data, setData] = useState<InsightsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('all');
  const [query, setQuery] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getInsights();
      setData(res.data);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const devices = data?.health || [];
  const filtered = useMemo(() => {
    return devices.filter((device) => {
      const matchesFilter = filter === 'all' || device.label === filter;
      const matchesQuery = !query || device.deviceName.toLowerCase().includes(query.toLowerCase());
      return matchesFilter && matchesQuery;
    });
  }, [devices, filter, query]);

  const avgScore = devices.length ? Math.round(devices.reduce((sum, item) => sum + item.score, 0) / devices.length) : 0;
  const critical = devices.filter((item) => item.score < 40).length;
  const watch = devices.filter((item) => item.score >= 40 && item.score < 70).length;
  const totalIssues = devices.reduce((sum, item) => sum + item.issues.filter((issue) => issue.type !== 'clear').length, 0);

  return (
    <div>
      <header className="mb-10 flex flex-col xl:flex-row xl:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">AI Health Score</h1>
          <p className="text-on-surface-variant font-body max-w-2xl">Device risk scoring from availability, latency, alerts, port changes, and traffic anomalies.</p>
        </div>
        <button onClick={load} className="bg-primary text-on-primary font-bold py-3 px-5 rounded-lg tracking-widest uppercase hover:brightness-110 active:scale-95 transition-all text-xs flex items-center justify-center gap-2">
          <span className="material-symbols-outlined text-base">refresh</span>
          Refresh
        </button>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-5 mb-6">
        <SummaryTile label="Average Score" value={avgScore} tone={scoreColor(avgScore)} />
        <SummaryTile label="Critical Devices" value={critical} tone="text-error" />
        <SummaryTile label="Watch List" value={watch} tone="text-amber-400" />
        <SummaryTile label="Open Issues" value={totalIssues} />
      </div>

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
      </div>

      {loading ? (
        <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 py-16 text-center text-on-surface-variant animate-pulse">Calculating health scores...</div>
      ) : filtered.length === 0 ? (
        <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 py-16 text-center text-on-surface-variant">No devices match this view</div>
      ) : (
        <div className="space-y-4">
          {filtered.map((device) => (
            <DeviceScoreRow
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
