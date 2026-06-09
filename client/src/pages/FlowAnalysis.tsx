import { useState, useEffect, useCallback } from 'react';
import {
  AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  PieChart, Pie, Cell, BarChart, Bar,
} from 'recharts';
import {
  getFlowStats, getFlowRecords, getTopTalkers,
  getProtocolDistribution, getFlowTimeseries
} from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint } from '../api/types';

const PROTOCOL_COLORS: Record<string, string> = {
  TCP: '#6ee7f7',
  UDP: '#d9fd3a',
  ICMP: '#f59e0b',
  IGMP: '#c084fc',
  GRE: '#fb923c',
  SCTP: '#4ade80',
  ESP: '#f472b6',
};

const CHART_COLORS = ['#6ee7f7', '#d9fd3a', '#f59e0b', '#c084fc', '#fb923c', '#4ade80', '#f472b6', '#ff7351'];

const TOOLTIP_STYLE = {
  background: '#1a1a13',
  border: '1px solid #494840',
  borderRadius: '8px',
  fontSize: '12px',
  color: '#f4f1e6',
};

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function StatCard({ label, value, icon, color = 'text-primary' }: { label: string; value: string | number; icon: string; color?: string }) {
  return (
    <div className="bg-surface-container-low p-5 rounded-xl border-l-2 border-primary/30 hover:border-primary/60 transition-all">
      <div className="flex items-center gap-2 mb-1">
        <span className="material-symbols-outlined text-sm opacity-60">{icon}</span>
        <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em]">{label}</p>
      </div>
      <p className={`font-headline text-2xl font-bold ${color}`}>{value}</p>
    </div>
  );
}

export default function FlowAnalysis() {
  const [stats, setStats] = useState<FlowStats | null>(null);
  const [timeseries, setTimeseries] = useState<FlowTimeseriesPoint[]>([]);
  const [topSources, setTopSources] = useState<TopTalker[]>([]);
  const [topDestinations, setTopDestinations] = useState<TopTalker[]>([]);
  const [protocols, setProtocols] = useState<ProtocolBreakdown[]>([]);
  const [flows, setFlows] = useState<FlowRecord[]>([]);
  const [flowFeed, setFlowFeed] = useState<Array<{ src: string; dst: string; proto: string; bytes: number; time: string }>>([]);
  const [filterIp, setFilterIp] = useState('');
  const [filterProto, setFilterProto] = useState('');

  const loadData = useCallback(async () => {
    try {
      const [statsRes, tsRes, srcRes, dstRes, protoRes, flowsRes] = await Promise.all([
        getFlowStats(),
        getFlowTimeseries({ bucketMinutes: 5 }),
        getTopTalkers({ limit: 10, direction: 'src' }),
        getTopTalkers({ limit: 10, direction: 'dst' }),
        getProtocolDistribution(),
        getFlowRecords({ limit: 50 }),
      ]);
      const rawStats = statsRes.data as unknown as Record<string, unknown> | null;
      if (rawStats) {
        setStats({
          totalFlows: Number(rawStats.totalFlows) || 0,
          totalBytes: Number(rawStats.totalBytes) || 0,
          totalBytesFormatted: formatBytes(Number(rawStats.totalBytes) || 0),
          totalPackets: Number(rawStats.totalPackets) || 0,
          uniqueSources: Number(rawStats.uniqueSources) || 0,
          uniqueDestinations: Number(rawStats.uniqueDestinations) || 0,
          activeCollectors: 0,
          collectorTypes: [],
        });
      }
      setTimeseries(tsRes.data || []);
      setTopSources(srcRes.data || []);
      setTopDestinations(dstRes.data || []);
      const rawProtos = protoRes.data as unknown as Record<string, number> | ProtocolBreakdown[] | null;
      if (rawProtos && !Array.isArray(rawProtos)) {
        const entries = Object.entries(rawProtos);
        const totalBytes = entries.reduce((s, [, v]) => s + (v as number), 0);
        setProtocols(entries.map(([name, bytes]) => ({
          protocolName: name,
          protocolNumber: 0,
          bytes: bytes as number,
          bytesFormatted: formatBytes(bytes as number),
          packets: 0,
          flows: 0,
          percentage: totalBytes > 0 ? Math.round(((bytes as number) / totalBytes) * 100) : 0,
        })));
      } else {
        setProtocols((rawProtos as ProtocolBreakdown[]) || []);
      }
      setFlows(flowsRes.data || []);
    } catch { /* handled by interceptor */ }
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  // Refresh data every 10 seconds
  useEffect(() => {
    const interval = setInterval(loadData, 10_000);
    return () => clearInterval(interval);
  }, [loadData]);

  useSocket({
    onFlowUpdate: (data) => {
      const d = data as { sample?: Array<{ src: string; dst: string; proto: string; bytes: number }>; count?: number };
      if (d.sample) {
        setFlowFeed((prev) => {
          const newItems = d.sample!.map((s) => ({ ...s, time: new Date().toLocaleTimeString() }));
          return [...newItems, ...prev].slice(0, 30);
        });
      }
      // Refresh stats periodically on flow events
      getFlowStats().then((r) => {
        const raw = r.data as unknown as Record<string, unknown> | null;
        if (raw) {
          setStats({
            totalFlows: Number(raw.totalFlows) || 0,
            totalBytes: Number(raw.totalBytes) || 0,
            totalBytesFormatted: formatBytes(Number(raw.totalBytes) || 0),
            totalPackets: Number(raw.totalPackets) || 0,
            uniqueSources: Number(raw.uniqueSources) || 0,
            uniqueDestinations: Number(raw.uniqueDestinations) || 0,
            activeCollectors: 0,
            collectorTypes: [],
          });
        }
      }).catch(() => {});
    },
  });

  const filteredFlows = flows.filter((f) => {
    if (filterIp && !f.srcIp.includes(filterIp) && !f.dstIp.includes(filterIp)) return false;
    if (filterProto && f.protocolName !== filterProto) return false;
    return true;
  });

  return (
    <div>
      {/* Header */}
      <header className="mb-10 flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">
            Flow Analysis
          </h1>
          <p className="text-on-surface-variant font-body max-w-xl">
            NetFlow / sFlow traffic analysis. Monitor bandwidth, top talkers, and protocol distribution in real-time.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2 h-2 rounded-full bg-[#6ee7f7] animate-pulse" />
          <span className="text-[#6ee7f7] font-mono text-xs">
            {stats ? `${stats.collectorTypes.length > 0 ? stats.collectorTypes.join(', ') : 'Awaiting flows'}` : 'Loading...'}
          </span>
        </div>
      </header>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Flows" value={stats ? formatNumber(stats.totalFlows) : '—'} icon="swap_horiz" />
        <StatCard label="Bandwidth" value={stats ? stats.totalBytesFormatted : '—'} icon="cloud_download" color="text-[#6ee7f7]" />
        <StatCard label="Unique Sources" value={stats ? formatNumber(stats.uniqueSources) : '—'} icon="upload" />
        <StatCard label="Unique Destinations" value={stats ? formatNumber(stats.uniqueDestinations) : '—'} icon="download" />
      </div>

      {/* Traffic Volume Timeline */}
      <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 mb-6">
        <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
          <span className="material-symbols-outlined text-[#6ee7f7] text-lg">show_chart</span>
          Traffic Volume Over Time
        </h3>
        {timeseries.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 opacity-50">
            <span className="material-symbols-outlined text-4xl mb-2">timeline</span>
            <p className="text-xs text-on-surface-variant uppercase tracking-widest">
              No flow data yet — configure a router to export NetFlow/sFlow to UDP port 2055
            </p>
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={240}>
            <AreaChart data={timeseries} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="gradBytes" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#6ee7f7" stopOpacity={0.4} />
                  <stop offset="95%" stopColor="#6ee7f7" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="gradPackets" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#d9fd3a" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#d9fd3a" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="timestamp"
                tick={{ fill: '#8a8a78', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fill: '#8a8a78', fontSize: 10 }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => formatBytes(v)}
                width={60}
              />
              <Tooltip
                contentStyle={TOOLTIP_STYLE}
                formatter={(value: unknown, name: unknown) => [
                  name === 'totalBytes' ? formatBytes(Number(value ?? 0)) : formatNumber(Number(value ?? 0)),
                  name === 'totalBytes' ? 'Bytes' : 'Packets'
                ]}
                labelFormatter={(l) => new Date(l).toLocaleString()}
              />
              <Legend
                wrapperStyle={{ fontSize: 11, paddingTop: 8 }}
                formatter={(value) => <span style={{ color: '#c8c5b0' }}>{value === 'totalBytes' ? 'Bytes' : 'Packets'}</span>}
              />
              <Area type="monotone" dataKey="totalBytes" stroke="#6ee7f7" fill="url(#gradBytes)" strokeWidth={2} />
              <Area type="monotone" dataKey="totalPackets" stroke="#d9fd3a" fill="url(#gradPackets)" strokeWidth={1.5} />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Top Talkers + Protocol Distribution */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        {/* Top Sources */}
        <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
            <span className="material-symbols-outlined text-[#d9fd3a] text-lg">upload</span>
            Top Sources
          </h3>
          {topSources.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-8 opacity-50">No data</p>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={topSources} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                <XAxis type="number" tick={{ fill: '#8a8a78', fontSize: 9 }} tickLine={false} axisLine={false} tickFormatter={(v) => formatBytes(v)} />
                <YAxis type="category" dataKey="ip" tick={{ fill: '#c8c5b0', fontSize: 10 }} tickLine={false} axisLine={false} width={110} />
                <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                <Bar dataKey="bytes" fill="#d9fd3a" radius={[0, 4, 4, 0]} barSize={16} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Top Destinations */}
        <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
            <span className="material-symbols-outlined text-[#6ee7f7] text-lg">download</span>
            Top Destinations
          </h3>
          {topDestinations.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-8 opacity-50">No data</p>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={topDestinations} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                <XAxis type="number" tick={{ fill: '#8a8a78', fontSize: 9 }} tickLine={false} axisLine={false} tickFormatter={(v) => formatBytes(v)} />
                <YAxis type="category" dataKey="ip" tick={{ fill: '#c8c5b0', fontSize: 10 }} tickLine={false} axisLine={false} width={110} />
                <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                <Bar dataKey="bytes" fill="#6ee7f7" radius={[0, 4, 4, 0]} barSize={16} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Protocol Distribution Donut */}
        <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 flex flex-col">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
            <span className="material-symbols-outlined text-[#c084fc] text-lg">pie_chart</span>
            Protocol Distribution
          </h3>
          {protocols.length === 0 ? (
            <p className="text-xs text-on-surface-variant text-center py-8 my-auto opacity-50">No data</p>
          ) : (
            <div className="flex flex-col items-center justify-center flex-1">
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={protocols}
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    paddingAngle={3}
                    dataKey="bytes"
                    nameKey="protocolName"
                    labelLine={false}
                  >
                    {protocols.map((entry, i) => (
                      <Cell key={entry.protocolName} fill={PROTOCOL_COLORS[entry.protocolName] || CHART_COLORS[i % CHART_COLORS.length]} stroke="transparent" />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={TOOLTIP_STYLE}
                    formatter={(v: unknown, name: unknown) => [formatBytes(Number(v ?? 0)), String(name)]}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
                {protocols.map((p, i) => (
                  <div key={p.protocolName} className="flex items-center gap-1.5 text-xs">
                    <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ background: PROTOCOL_COLORS[p.protocolName] || CHART_COLORS[i % CHART_COLORS.length] }} />
                    <span className="text-on-surface-variant">{p.protocolName}</span>
                    <span className="font-bold text-on-surface">{p.percentage}%</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Flow Records Table + Live Feed */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Flow Records Table */}
        <div className="xl:col-span-2 bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
          <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 mb-4">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest flex items-center gap-2">
              <span className="material-symbols-outlined text-primary text-lg">table_chart</span>
              Flow Records
            </h3>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="Filter IP..."
                value={filterIp}
                onChange={(e) => setFilterIp(e.target.value)}
                className="bg-surface-container-highest border border-outline-variant/30 rounded-lg px-3 py-1.5 text-xs text-on-surface placeholder:text-on-surface-variant/50 focus:outline-none focus:border-primary/50 w-32"
              />
              <select
                value={filterProto}
                onChange={(e) => setFilterProto(e.target.value)}
                className="bg-surface-container-highest border border-outline-variant/30 rounded-lg px-3 py-1.5 text-xs text-on-surface focus:outline-none focus:border-primary/50"
              >
                <option value="">All Protocols</option>
                {['TCP', 'UDP', 'ICMP', 'GRE', 'SCTP'].map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
            </div>
          </div>
          <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
            <table className="w-full text-left border-collapse">
              <thead className="sticky top-0 bg-surface-container-high z-10">
                <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
                  <th className="pb-2 font-medium">Time</th>
                  <th className="pb-2 font-medium">Source</th>
                  <th className="pb-2 font-medium">Destination</th>
                  <th className="pb-2 font-medium">Proto</th>
                  <th className="pb-2 font-medium text-right">Bytes</th>
                  <th className="pb-2 font-medium text-right">Packets</th>
                </tr>
              </thead>
              <tbody className="text-xs">
                {filteredFlows.length === 0 ? (
                  <tr><td colSpan={6} className="py-12 text-center text-on-surface-variant opacity-50">
                    <div className="flex flex-col items-center gap-2">
                      <span className="material-symbols-outlined text-3xl">swap_horiz</span>
                      <span className="uppercase tracking-widest text-[10px]">No flow records yet</span>
                    </div>
                  </td></tr>
                ) : (
                  filteredFlows.map((f) => {
                    const protoColor = PROTOCOL_COLORS[f.protocolName] || '#8a8a78';
                    return (
                      <tr key={f.id} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-colors">
                        <td className="py-2.5 text-on-surface-variant font-mono">
                          {new Date(f.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                        </td>
                        <td className="py-2.5 font-mono text-on-surface">{f.srcIp}{f.srcPort ? `:${f.srcPort}` : ''}</td>
                        <td className="py-2.5 font-mono text-on-surface">{f.dstIp}{f.dstPort ? `:${f.dstPort}` : ''}</td>
                        <td className="py-2.5">
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider border"
                            style={{ color: protoColor, borderColor: `${protoColor}33`, backgroundColor: `${protoColor}15` }}>
                            {f.protocolName}
                          </span>
                        </td>
                        <td className="py-2.5 text-right font-mono text-on-surface">{formatBytes(f.bytes)}</td>
                        <td className="py-2.5 text-right font-mono text-on-surface-variant">{formatNumber(f.packets)}</td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Live Flow Feed */}
        <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
          <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-[#6ee7f7] animate-pulse" />
            Live Flow Feed
          </h3>
          <div className="space-y-2 max-h-[400px] overflow-y-auto">
            {flowFeed.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 opacity-50">
                <span className="material-symbols-outlined text-3xl mb-2">rss_feed</span>
                <p className="text-[10px] text-on-surface-variant uppercase tracking-widest">Waiting for flow data...</p>
              </div>
            ) : (
              flowFeed.map((f, i) => (
                <div key={i} className="flex items-center gap-3 p-2.5 rounded-lg bg-surface-container-highest/50 border border-outline-variant/10 hover:border-[#6ee7f7]/20 transition-all text-xs">
                  <span className="text-[10px] text-on-surface-variant font-mono w-14 flex-shrink-0">{f.time}</span>
                  <span className="font-mono text-on-surface truncate flex-1">{f.src}</span>
                  <span className="material-symbols-outlined text-[12px] text-on-surface-variant">arrow_forward</span>
                  <span className="font-mono text-on-surface truncate flex-1">{f.dst}</span>
                  <span className="inline-flex px-1.5 py-0.5 rounded text-[9px] font-bold uppercase"
                    style={{ color: PROTOCOL_COLORS[f.proto] || '#8a8a78', background: `${(PROTOCOL_COLORS[f.proto] || '#8a8a78')}15` }}>
                    {f.proto}
                  </span>
                  <span className="font-mono text-on-surface-variant text-right w-16 flex-shrink-0">{formatBytes(f.bytes)}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
