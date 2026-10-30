import { useEffect, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { useQuery } from '@tanstack/react-query';
import { getISPLinkSLA, getISPLinkTimeseries, type ISPLink } from '../api/isp';
import { queryKeys } from '../api/queryKeys';
import { TOOLTIP_STYLE } from '../utils/chartConfig';
import { formatMbps } from '../utils/formatters';

export default function ISPLinkModal({ link, onClose }: { link: ISPLink; onClose: () => void }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const onCloseRef = useRef(onClose);
  useEffect(() => { onCloseRef.current = onClose; });
  const slaQuery = useQuery({
    queryKey: queryKeys.isp.sla(link.id),
    queryFn: ({ signal }) => getISPLinkSLA(link.id, { signal }),
  });
  const timeseriesQuery = useQuery({
    queryKey: queryKeys.isp.timeseries(link.id),
    queryFn: ({ signal }) => getISPLinkTimeseries(link.id, { signal }),
  });
  const loading = slaQuery.isPending || timeseriesQuery.isPending;
  const failure = slaQuery.error ?? timeseriesQuery.error;
  const error = failure ? (failure instanceof Error ? failure.message : 'Failed to load ISP data') : null;
  const sla = slaQuery.data?.data ?? null;
  const timeSeries = timeseriesQuery.data?.data || [];

  useEffect(() => {
    previousFocus.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { onCloseRef.current(); return; }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => { document.removeEventListener('keydown', handleKeyDown); previousFocus.current?.focus(); };
  }, []);

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
  const slaColor = slaMet === true ? 'text-success' : slaMet === false ? 'text-error' : 'text-fg-subtle';

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 pt-16 bg-black/60" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`ISP link details for ${link.name}`}
        tabIndex={-1}
        className="bg-surface-1 border border-line rounded-lg w-full max-w-4xl max-h-[calc(100vh-4rem)] overflow-hidden flex flex-col outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-6 border-b border-line flex justify-between items-start shrink-0">
          <div>
            <h2 className="font-sans text-3xl font-semibold text-fg">{link.name}</h2>
            <p className="text-fg-muted text-sm font-mono mt-0.5">{link.provider} · {link.gateway_ip} · #{link.id}</p>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-inset rounded-full transition-colors" aria-label="Close dialog">
            <span className="material-symbols-outlined text-fg-subtle hover:text-fg">close</span>
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 min-h-0">
          {loading ? (
            <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading ISP data...</div>
          ) : error ? (
            <div className="h-64 flex flex-col items-center justify-center text-error text-sm gap-2">
              <span className="material-symbols-outlined text-3xl">error</span>
              <p>{error}</p>
              <button onClick={() => { void slaQuery.refetch(); void timeseriesQuery.refetch(); }} className="text-xs text-primary hover:underline">Retry</button>
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Status</p>
                  <p className={`font-semibold uppercase ${statusColor(sla?.latestStatus || 'unknown')}`}>{sla?.latestStatus || 'UNKNOWN'}</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Uptime</p>
                  <p className={`font-semibold ${slaColor}`}>{sla?.actualUptime != null ? `${sla.actualUptime.toFixed(2)}%` : '-'}</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Latency</p>
                  <p className="font-semibold text-fg">{sla?.avgLatencyMs != null ? `${sla.avgLatencyMs.toFixed(1)}ms` : '-'}</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Packet loss</p>
                  <p className={`font-semibold ${sla && sla.avgPacketLoss > 1 ? 'text-error' : 'text-success'}`}>{sla?.avgPacketLoss != null ? `${sla.avgPacketLoss.toFixed(2)}%` : '-'}</p>
                </div>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Bandwidth</p>
                  <p className="font-semibold text-fg">{link.bandwidth_mbps} Mbps</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">SLA target</p>
                  <p className="font-semibold text-fg">{link.sla_uptime_percent || 99.5}%</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Jitter</p>
                  <p className="font-semibold text-fg">{sla?.avgJitterMs != null ? `${sla.avgJitterMs.toFixed(1)}ms` : '-'}</p>
                </div>
                <div className="bg-surface-1 p-4 rounded-lg">
                  <p className="text-xs text-fg-muted mb-1">Total probes</p>
                  <p className="font-semibold text-fg">{sla?.totalProbes ?? 0}</p>
                </div>
              </div>

              <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="text-sm font-sans font-semibold">Latency & jitter</h3>
                  <span className={`text-xs font-semibold uppercase px-2 py-0.5 rounded-full ${slaMet === true ? 'bg-success/10 text-success' : slaMet === false ? 'bg-error/10 text-error' : 'bg-inset text-fg-subtle'}`}>
                    {slaMet === true ? 'SLA Met' : slaMet === false ? 'SLA Breached' : 'No SLA'}
                  </span>
                </div>
                {chartData.length === 0 ? (
                  <div className="h-64 flex items-center justify-center text-fg-muted text-sm">No metrics data yet. Probes run every {link.monitoring_interval_seconds}s.</div>
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

              <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
                <h3 className="text-sm font-sans font-semibold mb-4">Packet loss</h3>
                {chartData.length === 0 ? (
                  <div className="h-40 flex items-center justify-center text-fg-muted text-sm">No data</div>
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
                <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
                  <h3 className="text-sm font-sans font-semibold mb-4">Throughput</h3>
                  {chartData.length === 0 ? (
                    <div className="h-40 flex items-center justify-center text-fg-muted text-sm">No data</div>
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
                <div className="bg-surface-1 rounded-lg p-4 border border-line">
                  <h3 className="text-sm font-sans font-semibold mb-3">SLA compliance</h3>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-xs text-fg-muted">Target uptime</p>
                      <p className="text-lg font-semibold text-fg">{sla.slaTarget}%</p>
                    </div>
                    <div>
                      <p className="text-xs text-fg-muted">Actual uptime</p>
                      <p className={`text-lg font-semibold ${slaColor}`}>{sla.actualUptime.toFixed(2)}%</p>
                    </div>
                    {sla.slaGap != null && (
                      <>
                        <div>
                          <p className="text-xs text-fg-muted">Gap</p>
                          <p className={`text-lg font-semibold ${sla.slaGap >= 0 ? 'text-success' : 'text-error'}`}>{sla.slaGap >= 0 ? '+' : ''}{sla.slaGap.toFixed(2)}%</p>
                        </div>
                        <div>
                          <p className="text-xs text-fg-muted">Monthly cost</p>
                          <p className="text-lg font-semibold text-fg">{link.cost_monthly > 0 ? `₹${link.cost_monthly.toLocaleString()}` : '-'}</p>
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
