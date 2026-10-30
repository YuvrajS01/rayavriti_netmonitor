import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
  PieChart, Pie, Cell, BarChart, Bar,
} from 'recharts';
import {
  getFlowStats, getFlowRecords, getTopTalkers,
  getProtocolDistribution, getFlowTimeseries
} from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useSocket } from '../hooks/useSocket';
import type { FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint } from '../api/types';

const EMPTY_TIMESERIES: FlowTimeseriesPoint[] = [];
const EMPTY_TALKERS: TopTalker[] = [];
const EMPTY_PROTOCOLS: ProtocolBreakdown[] = [];
const EMPTY_FLOWS: FlowRecord[] = [];
import { PROTOCOL_COLORS, CHART_COLORS, TOOLTIP_STYLE, AXIS_TICK_STYLE, LEGEND_STYLE } from '../utils/chartConfig';
import { formatBytes, formatNumber } from '../utils/formatters';
import StatCard from '../components/ui/StatCard';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';
import SectionHeader from '../components/ui/SectionHeader';
import Treemap from '../components/charts/Treemap';
import SankeyDiagram from '../components/charts/SankeyDiagram';

const FLOW_POLL_MS = 10_000;

export default function FlowAnalysis() {
  const [flowFeed, setFlowFeed] = useState<Array<{ src: string; dst: string; proto: string; bytes: number; time: string }>>([]);
  const [filterIp, setFilterIp] = useState('');
  const [filterProto, setFilterProto] = useState('');

  // Independent queries: one failing endpoint no longer blanks the page.
  // refetchInterval pauses while the tab is hidden.
  const poll = { refetchInterval: FLOW_POLL_MS } as const;
  const statsQuery = useQuery({ queryKey: queryKeys.flows.stats(), queryFn: ({ signal }) => getFlowStats({ signal }), ...poll });
  const timeseriesQuery = useQuery({ queryKey: queryKeys.flows.timeseries(5), queryFn: ({ signal }) => getFlowTimeseries({ bucketMinutes: 5 }, { signal }), ...poll });
  const sourcesQuery = useQuery({ queryKey: queryKeys.flows.topTalkers('src', 10), queryFn: ({ signal }) => getTopTalkers({ limit: 10, direction: 'src' }, { signal }), ...poll });
  const destinationsQuery = useQuery({ queryKey: queryKeys.flows.topTalkers('dst', 10), queryFn: ({ signal }) => getTopTalkers({ limit: 10, direction: 'dst' }, { signal }), ...poll });
  const protocolsQuery = useQuery({ queryKey: queryKeys.flows.protocols(), queryFn: ({ signal }) => getProtocolDistribution({}, { signal }), ...poll });
  const flowsQuery = useQuery({ queryKey: queryKeys.flows.records(50), queryFn: ({ signal }) => getFlowRecords({ limit: 50 }, { signal }), ...poll });

  const queries = [statsQuery, timeseriesQuery, sourcesQuery, destinationsQuery, protocolsQuery, flowsQuery];
  const loading = queries.every((q) => q.isPending);
  const error = queries.every((q) => q.isError) ? 'Failed to load flow data. Please try again.' : null;
  const retry = () => { for (const q of queries) void q.refetch(); };

  const stats: FlowStats | null = statsQuery.data?.data ?? null;
  const timeseries = timeseriesQuery.data?.data || EMPTY_TIMESERIES;
  const topSources = sourcesQuery.data?.data || EMPTY_TALKERS;
  const topDestinations = destinationsQuery.data?.data || EMPTY_TALKERS;
  const protocols = protocolsQuery.data?.data || EMPTY_PROTOCOLS;
  const flows = flowsQuery.data?.data || EMPTY_FLOWS;

  useSocket({
    onFlowUpdate: (d) => {
      const sample = d.sample;
      if (sample) {
        setFlowFeed((prev) => {
          const newItems = sample.map((s) => ({ ...s, time: new Date().toLocaleTimeString() }));
          return [...newItems, ...prev].slice(0, 30);
        });
      }
    },
  });

  const filteredFlows = useMemo(() => flows.filter((f) => {
    if (filterIp && !f.srcIp.includes(filterIp) && !f.dstIp.includes(filterIp)) return false;
    if (filterProto && f.protocol !== filterProto) return false;
    return true;
  }), [flows, filterIp, filterProto]);

  return (
    <div>
      <SectionHeader
        title="Flow Analysis"
        subtitle="Analyze bandwidth, top talkers, and protocol distribution in real-time."
        action={
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-info" />
            <span className="text-info font-data text-xs">
              {loading ? 'Connecting...' : stats ? (stats.totalFlows > 0 ? 'Flows active' : 'Awaiting flows') : 'No data'}
            </span>
          </div>
        }
      />

      {loading && <LoadingState message="Loading flow data..." />}

      {error && !loading && <ErrorState message={error} onRetry={retry} />}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <StatCard label="Total Flows" value={stats ? formatNumber(stats.totalFlows) : '—'} icon="swap_horiz" />
            <StatCard label="Bandwidth" value={stats ? formatBytes(stats.totalBytes) : '—'} icon="cloud_download" color="text-info" />
            <StatCard label="Unique Sources" value={stats ? formatNumber(stats.uniqueSources) : '—'} icon="upload" />
            <StatCard label="Unique Destinations" value={stats ? formatNumber(stats.uniqueDestinations) : '—'} icon="download" />
          </div>

          <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20 mb-6">
            <h3 className="text-sm font-headline font-semibold uppercase tracking-wide mb-4 flex items-center gap-2">
              <span className="material-symbols-outlined text-info text-lg">show_chart</span>
              Traffic Volume Over Time
            </h3>
            {timeseries.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16">
                <span className="material-symbols-outlined text-4xl mb-2">timeline</span>
                <p className="text-xs text-on-surface-variant uppercase tracking-wide">
                  No flow data yet — configure a router to export NetFlow/sFlow to UDP port 2055
                </p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={240}>
                <AreaChart data={timeseries} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="gradBytes" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#6bb8c9" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#6bb8c9" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradPackets" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#d9fd3a" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#d9fd3a" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <XAxis
                    dataKey="bucketTime"
                    tick={AXIS_TICK_STYLE}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v) => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={AXIS_TICK_STYLE}
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
                    wrapperStyle={LEGEND_STYLE}
                    formatter={(value) => <span style={{ color: '#adaba1' }}>{value === 'totalBytes' ? 'Bytes' : 'Packets'}</span>}
                  />
                  <Area type="monotone" dataKey="totalBytes" stroke="#6bb8c9" fill="url(#gradBytes)" strokeWidth={2} />
                  <Area type="monotone" dataKey="totalPackets" stroke="#d9fd3a" fill="url(#gradPackets)" strokeWidth={1.5} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
            <SankeyDiagram flows={flows.map((f) => ({ src: f.srcIp, dst: f.dstIp, bytes: f.bytes, protocol: f.protocol }))} />
            <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
              <h3 className="text-sm font-headline font-semibold uppercase tracking-wide mb-4 flex items-center gap-2">
                <span className="material-symbols-outlined text-[#d9fd3a] text-lg">upload</span>
                Top Sources
              </h3>
              {topSources.length === 0 ? (
                <p className="text-xs text-on-surface-variant text-center py-8">No data</p>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={topSources} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                    <XAxis type="number" tick={{ fill: '#77766d', fontSize: 9 }} tickLine={false} axisLine={false} tickFormatter={(v) => formatBytes(v)} />
                    <YAxis type="category" dataKey="ip" tick={{ fill: '#adaba1', fontSize: 10 }} tickLine={false} axisLine={false} width={110} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                    <Bar dataKey="count" fill="#d9fd3a" radius={[0, 4, 4, 0]} barSize={16} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
              <h3 className="text-sm font-headline font-semibold uppercase tracking-wide mb-4 flex items-center gap-2">
                <span className="material-symbols-outlined text-info text-lg">download</span>
                Top Destinations
              </h3>
              {topDestinations.length === 0 ? (
                <p className="text-xs text-on-surface-variant text-center py-8">No data</p>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={topDestinations} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                    <XAxis type="number" tick={{ fill: '#77766d', fontSize: 9 }} tickLine={false} axisLine={false} tickFormatter={(v) => formatBytes(v)} />
                    <YAxis type="category" dataKey="ip" tick={{ fill: '#adaba1', fontSize: 10 }} tickLine={false} axisLine={false} width={110} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                    <Bar dataKey="count" fill="#6bb8c9" radius={[0, 4, 4, 0]} barSize={16} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <Treemap title="Protocol bandwidth" items={protocols.map((p, i) => ({ label: p.protocolName, value: p.bytes, color: PROTOCOL_COLORS[p.protocolName] || CHART_COLORS[i % CHART_COLORS.length], detail: p.bytesFormatted }))} />
            <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20 flex flex-col">
              <h3 className="text-sm font-headline font-semibold uppercase tracking-wide mb-4 flex items-center gap-2">
                <span className="material-symbols-outlined text-[#c084fc] text-lg">pie_chart</span>
                Protocol Distribution
              </h3>
              {protocols.length === 0 ? (
                <p className="text-xs text-on-surface-variant text-center py-8 my-auto">No data</p>
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
                        <span className="font-semibold text-on-surface">{p.percentage}%</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
            <div className="xl:col-span-2 bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
              <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 mb-4">
                <h3 className="text-sm font-headline font-semibold uppercase tracking-wide flex items-center gap-2">
                  <span className="material-symbols-outlined text-primary text-lg">table_chart</span>
                  Flow Records
                </h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    placeholder="Filter IP..."
                    value={filterIp}
                    onChange={(e) => setFilterIp(e.target.value)}
                    className="bg-surface-container-lowest border border-outline-variant/30 rounded-lg px-3 py-1.5 text-xs text-on-surface placeholder:text-on-surface-variant/50 focus:outline-none focus:border-primary/50 w-32"
                  />
                  <select
                    value={filterProto}
                    onChange={(e) => setFilterProto(e.target.value)}
                    className="bg-surface-container-lowest border border-outline-variant/30 rounded-lg px-3 py-1.5 text-xs text-on-surface focus:outline-none focus:border-primary/50"
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
                  <thead className="sticky top-0 bg-surface-container-low z-10">
                    <tr className="text-[10px] uppercase tracking-wide text-on-surface-variant border-b border-outline-variant/20">
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
                      <tr><td colSpan={6} className="py-12 text-center text-on-surface-variant">
                        <div className="flex flex-col items-center gap-2">
                          <span className="material-symbols-outlined text-3xl">swap_horiz</span>
                          <span className="uppercase tracking-wide text-[10px]">No flow records yet</span>
                        </div>
                      </td></tr>
                    ) : (
                      filteredFlows.map((f) => {
                        const protoColor = PROTOCOL_COLORS[f.protocol] || '#77766d';
                        return (
                          <tr key={f.id} className="border-b border-outline-variant/10 hover:bg-surface-container-lowest/50 transition-colors">
                            <td className="py-2.5 text-on-surface-variant font-data">
                              {new Date(f.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                            </td>
                            <td className="py-2.5 font-data text-on-surface">{f.srcIp}{f.srcPort ? `:${f.srcPort}` : ''}</td>
                            <td className="py-2.5 font-data text-on-surface">{f.dstIp}{f.dstPort ? `:${f.dstPort}` : ''}</td>
                            <td className="py-2.5">
                              <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider border"
                                style={{ color: protoColor, borderColor: `${protoColor}33`, backgroundColor: `${protoColor}15` }}>
                                {f.protocol}
                              </span>
                            </td>
                            <td className="py-2.5 text-right font-data text-on-surface">{formatBytes(f.bytes)}</td>
                            <td className="py-2.5 text-right font-data text-on-surface-variant">{formatNumber(f.packets)}</td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
              <h3 className="text-sm font-headline font-semibold uppercase tracking-wide mb-4 flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-info" />
                Live Flow Feed
              </h3>
              <div className="space-y-2 max-h-[400px] overflow-y-auto">
                {flowFeed.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12">
                    <span className="material-symbols-outlined text-3xl mb-2">rss_feed</span>
                    <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Waiting for flow data...</p>
                  </div>
                ) : (
                  flowFeed.map((f, i) => (
                    <div key={i} className="flex items-center gap-3 p-2.5 rounded-lg bg-surface-container-lowest/50 border border-outline-variant/10 hover:border-info/20 transition-colors duration-200 text-xs">
                      <span className="text-[10px] text-on-surface-variant font-data w-14 flex-shrink-0">{f.time}</span>
                      <span className="font-data text-on-surface truncate flex-1">{f.src}</span>
                      <span className="material-symbols-outlined text-[12px] text-on-surface-variant">arrow_forward</span>
                      <span className="font-data text-on-surface truncate flex-1">{f.dst}</span>
                      <span className="inline-flex px-1.5 py-0.5 rounded text-[9px] font-semibold uppercase"
                        style={{ color: PROTOCOL_COLORS[f.proto] || '#77766d', background: `${(PROTOCOL_COLORS[f.proto] || '#77766d')}15` }}>
                        {f.proto}
                      </span>
                      <span className="font-data text-on-surface-variant text-right w-16 flex-shrink-0">{formatBytes(f.bytes)}</span>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
