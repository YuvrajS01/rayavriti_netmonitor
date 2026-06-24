import { useState, useEffect, useRef, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { v1, wrap } from '../api/http';
import { TOOLTIP_STYLE } from '../utils/chartConfig';
import { formatMbps } from '../utils/formatters';

export interface ISPLink {
  id: number;
  name: string;
  provider: string;
  circuit_id: string;
  bandwidth_mbps: number;
  gateway_ip: string;
  sla_uptime_percent: number;
  cost_monthly: number;
  monitoring_interval_seconds: number;
  enabled: boolean;
}

interface ISPSLA {
  linkId: number;
  name: string;
  provider: string;
  slaTarget: number | null;
  actualUptime: number;
  slaCompliant: boolean;
  slaGap: number | null;
  avgLatencyMs: number;
  avgJitterMs: number;
  avgPacketLoss: number;
  totalProbes: number;
  latestStatus: string;
}

interface ISPTimeSeriesPoint {
  timestamp: string;
  latencyMs: number | null;
  jitterMs: number | null;
  packetLoss: number | null;
  downloadMbps: number | null;
  uploadMbps: number | null;
  status: string;
}

export default function ISPLinkModal({ link, onClose }: { link: ISPLink; onClose: () => void }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const [sla, setSLA] = useState<ISPSLA | null>(null);
  const [timeSeries, setTimeSeries] = useState<ISPTimeSeriesPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    previousFocus.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { onCloseRef.current(); return; }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => { document.removeEventListener('keydown', handleKeyDown); previousFocus.current?.focus(); };
  }, []);

  const loadData = useCallback(async () => {
    try {
      const [slaRes, tsRes] = await Promise.all([
        v1.get(`/isp-links/${link.id}/sla`),
        v1.get(`/isp-links/${link.id}/metrics/timeseries`),
      ]);
      setSLA(wrap<ISPSLA>(slaRes.data).data);
      setTimeSeries(wrap<ISPTimeSeriesPoint[]>(tsRes.data).data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load ISP data');
    } finally {
      setLoading(false);
    }
  }, [link.id]);

  useEffect(() => { void Promise.resolve().then(loadData); }, [loadData]);

  const chartData = timeSeries.map((p) => ({
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    latency: p.latencyMs ?? 0,
    jitter: p.jitterMs ?? 0,
    packetLoss: p.packetLoss ?? 0,
    download: p.downloadMbps ?? 0,
    upload: p.uploadMbps ?? 0,
  })).reverse();

  const statusColor = (s: string) => {
    if (s === 'up') return 'text-success';
    if (s === 'degraded') return 'text-warning';
    return 'text-error';
  };

  const slaMet = sla?.slaCompliant;
  const slaColor = slaMet === true ? 'text-success' : slaMet === false ? 'text-error' : 'text-outline';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`ISP link details for ${link.name}`}
        tabIndex={-1}
        className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-3xl max-h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-start shrink-0">
          <div>
            <h2 className="font-headline text-3xl font-bold text-on-surface uppercase tracking-tight">{link.name}</h2>
            <p className="text-on-surface-variant text-sm font-mono mt-0.5">{link.provider} · {link.gateway_ip} · #{link.id}</p>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-colors" aria-label="Close dialog">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 min-h-0">
          {loading ? (
            <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">Loading ISP data...</div>
          ) : error ? (
            <div className="h-64 flex flex-col items-center justify-center text-error text-sm gap-2">
              <span className="material-symbols-outlined text-3xl">error</span>
              <p>{error}</p>
              <button onClick={() => { setError(null); setLoading(true); loadData(); }} className="text-xs text-primary hover:underline">Retry</button>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Status</p>
                  <p className={`font-bold uppercase ${statusColor(sla?.latestStatus || 'unknown')}`}>{sla?.latestStatus || 'UNKNOWN'}</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Uptime</p>
                  <p className={`font-bold ${slaColor}`}>{sla?.actualUptime != null ? `${sla.actualUptime.toFixed(2)}%` : '-'}</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Latency</p>
                  <p className="font-bold text-on-surface">{sla?.avgLatencyMs != null ? `${sla.avgLatencyMs.toFixed(1)}ms` : '-'}</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Packet Loss</p>
                  <p className={`font-bold ${sla && sla.avgPacketLoss > 1 ? 'text-error' : 'text-success'}`}>{sla?.avgPacketLoss != null ? `${sla.avgPacketLoss.toFixed(2)}%` : '-'}</p>
                </div>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Bandwidth</p>
                  <p className="font-bold text-on-surface">{link.bandwidth_mbps} Mbps</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">SLA Target</p>
                  <p className="font-bold text-on-surface">{link.sla_uptime_percent || 99.5}%</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Jitter</p>
                  <p className="font-bold text-on-surface">{sla?.avgJitterMs != null ? `${sla.avgJitterMs.toFixed(1)}ms` : '-'}</p>
                </div>
                <div className="bg-surface-container-high p-4 rounded-lg">
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Total Probes</p>
                  <p className="font-bold text-on-surface">{sla?.totalProbes ?? 0}</p>
                </div>
              </div>

              <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-sm font-headline font-bold uppercase tracking-wide">Latency & Jitter</h3>
                  <span className={`text-xs font-bold uppercase px-2 py-0.5 rounded-full ${slaMet === true ? 'bg-success/10 text-success' : slaMet === false ? 'bg-error/10 text-error' : 'bg-surface-container-highest text-outline'}`}>
                    {slaMet === true ? 'SLA Met' : slaMet === false ? 'SLA Breached' : 'No SLA'}
                  </span>
                </div>
                {chartData.length === 0 ? (
                  <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">No metrics data yet. Probes run every {link.monitoring_interval_seconds}s.</div>
                ) : (
                  <ResponsiveContainer width="100%" height={256}>
                    <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                      <XAxis dataKey="time" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} />
                      <YAxis tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                      <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(value: unknown, name: unknown) => [`${Number(value ?? 0).toFixed(1)}ms`, String(name) === 'latency' ? 'Latency' : 'Jitter']} />
                      <Legend wrapperStyle={{ fontSize: 11, color: '#c9c6b8' }} />
                      <Line name="Latency" type="monotone" dataKey="latency" stroke="#d9fd3a" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                      <Line name="Jitter" type="monotone" dataKey="jitter" stroke="#7dd3fc" strokeWidth={1.5} dot={false} activeDot={{ r: 3 }} connectNulls />
                    </LineChart>
                  </ResponsiveContainer>
                )}
              </div>

              <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
                <h3 className="text-sm font-headline font-bold mb-4 uppercase tracking-wide">Packet Loss</h3>
                {chartData.length === 0 ? (
                  <div className="h-40 flex items-center justify-center text-on-surface-variant text-sm">No data</div>
                ) : (
                  <ResponsiveContainer width="100%" height={160}>
                    <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                      <XAxis dataKey="time" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} />
                      <YAxis tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}%`} domain={[0, 'auto']} />
                      <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(value: unknown) => [`${Number(value ?? 0).toFixed(2)}%`, 'Packet Loss']} />
                      <Line type="monotone" dataKey="packetLoss" stroke="#f87171" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                    </LineChart>
                  </ResponsiveContainer>
                )}
              </div>

              {link.bandwidth_mbps > 0 && (
                <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
                  <h3 className="text-sm font-headline font-bold mb-4 uppercase tracking-wide">Throughput</h3>
                  {chartData.length === 0 ? (
                    <div className="h-40 flex items-center justify-center text-on-surface-variant text-sm">No data</div>
                  ) : (
                    <ResponsiveContainer width="100%" height={160}>
                      <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                        <XAxis dataKey="time" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} />
                        <YAxis tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}M`} />
                        <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(value: unknown, name: unknown) => [formatMbps(Number(value ?? 0)), String(name) === 'download' ? 'Download' : 'Upload']} />
                        <Legend wrapperStyle={{ fontSize: 11, color: '#c9c6b8' }} />
                        <Line name="Download" type="monotone" dataKey="download" stroke="#d9fd3a" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                        <Line name="Upload" type="monotone" dataKey="upload" stroke="#7dd3fc" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                      </LineChart>
                    </ResponsiveContainer>
                  )}
                </div>
              )}

              {sla && sla.slaTarget != null && (
                <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20">
                  <h3 className="text-sm font-headline font-bold mb-3 uppercase tracking-wide">SLA Compliance</h3>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Target Uptime</p>
                      <p className="text-lg font-bold text-on-surface">{sla.slaTarget}%</p>
                    </div>
                    <div>
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Actual Uptime</p>
                      <p className={`text-lg font-bold ${slaColor}`}>{sla.actualUptime.toFixed(2)}%</p>
                    </div>
                    {sla.slaGap != null && (
                      <>
                        <div>
                          <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Gap</p>
                          <p className={`text-lg font-bold ${sla.slaGap >= 0 ? 'text-success' : 'text-error'}`}>{sla.slaGap >= 0 ? '+' : ''}{sla.slaGap.toFixed(2)}%</p>
                        </div>
                        <div>
                          <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Monthly Cost</p>
                          <p className="text-lg font-bold text-on-surface">{link.cost_monthly > 0 ? `₹${link.cost_monthly.toLocaleString()}` : '-'}</p>
                        </div>
                      </>
                    )}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
