import { useState, useMemo, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  AreaChart, Area, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend,
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
import { PROTOCOL_COLORS, CHART_COLORS, SERIES_COLORS, TOOLTIP_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_ITEM_STYLE, AXIS_TICK_STYLE, GRID_COLOR, LEGEND_STYLE, legendFormatter } from '../utils/chartConfig';
import { useChartAnimation } from '../hooks/useReducedMotion';
import { StatusDot } from '../components/ui/Status';
import { asText, formatBytes, formatNumber } from '../utils/formatters';
import StatCard from '../components/ui/StatCard';
import LoadingState from '../components/ui/LoadingState';
import ErrorState from '../components/ui/ErrorState';
import SectionHeader from '../components/ui/SectionHeader';
import Treemap from '../components/charts/Treemap';
import SankeyDiagram from '../components/charts/SankeyDiagram';

const BYTES_COLOR = SERIES_COLORS[0];
const PACKETS_COLOR = SERIES_COLORS[1];
const CATEGORY_TICK = { ...AXIS_TICK_STYLE, fill: 'var(--color-fg-muted)', fontFamily: 'var(--font-mono)' };

const protocolColor = (proto: string, index = 0) => PROTOCOL_COLORS[proto] || CHART_COLORS[index % CHART_COLORS.length] || 'var(--color-fg-subtle)';

/** Protocol tag: neutral text with a coloured identity dot (text never wears a series colour). */
function ProtoChip({ proto }: { proto: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-sm border border-line bg-surface-2 px-1.5 py-0.5 text-xs font-semibold uppercase text-fg">
      <span className="h-2 w-2 rounded-full" style={{ background: PROTOCOL_COLORS[proto] || 'var(--color-fg-subtle)' }} aria-hidden="true" />
      {proto}
    </span>
  );
}

const FLOW_POLL_MS = 10_000;

export default function FlowAnalysis() {
  const [flowFeed, setFlowFeed] = useState<Array<{ src: string; dst: string; proto: string; bytes: number; time: string }>>([]);
  const [filterIp, setFilterIp] = useState('');
  const [filterProto, setFilterProto] = useState('');
  const anim = useChartAnimation();

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
        title="Flow analysis"
        subtitle="Bandwidth, top talkers and protocol distribution in real time."
        action={
          <div className="flex items-center gap-2 rounded-md border border-line bg-surface-1 px-3 py-1.5 text-xs text-fg-muted" role="status">
            <StatusDot status={stats && stats.totalFlows > 0 ? 'up' : 'unknown'} size={8} label="" />
            {loading ? 'Connecting…' : stats ? (stats.totalFlows > 0 ? 'Flows active' : 'Awaiting flows') : 'No data'}
          </div>
        }
      />

      {loading && <LoadingState message="Loading flow data..." />}

      {error && !loading && <ErrorState message={error} onRetry={retry} />}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <StatCard label="Total flows" value={stats ? formatNumber(stats.totalFlows) : '—'} icon="swap_horiz" />
            <StatCard label="Bandwidth" value={stats ? formatBytes(stats.totalBytes) : '—'} icon="cloud_download" />
            <StatCard label="Unique sources" value={stats ? formatNumber(stats.uniqueSources) : '—'} icon="upload" />
            <StatCard label="Unique destinations" value={stats ? formatNumber(stats.uniqueDestinations) : '—'} icon="download" />
          </div>

          <div className="bg-surface-1 rounded-lg p-5 border border-line mb-6">
            <h3 className="text-sm font-semibold text-fg mb-4">Traffic volume over time</h3>
            {timeseries.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16">
                <span className="material-symbols-outlined text-4xl mb-2 text-fg-subtle" aria-hidden="true">timeline</span>
                <p className="text-sm text-fg-muted text-center">
                  No flow data yet — configure a router to export NetFlow/sFlow to UDP port 2055
                </p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={240}>
                <AreaChart data={timeseries} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="gradBytes" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor={BYTES_COLOR} stopOpacity={0.25} />
                      <stop offset="95%" stopColor={BYTES_COLOR} stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="gradPackets" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor={PACKETS_COLOR} stopOpacity={0.2} />
                      <stop offset="95%" stopColor={PACKETS_COLOR} stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid stroke={GRID_COLOR} strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="bucketTime"
                    tick={AXIS_TICK_STYLE}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v: string) => new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={AXIS_TICK_STYLE}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v: number) => formatBytes(v)}
                    width={60}
                  />
                  <Tooltip
                    contentStyle={TOOLTIP_STYLE}
                    labelStyle={TOOLTIP_LABEL_STYLE}
                    itemStyle={TOOLTIP_ITEM_STYLE}
                    formatter={(value: unknown, name: unknown) => [
                      name === 'totalBytes' ? formatBytes(Number(value ?? 0)) : formatNumber(Number(value ?? 0)),
                      name === 'totalBytes' ? 'Bytes' : 'Packets'
                    ]}
                    labelFormatter={(l: ReactNode) => new Date(asText(l)).toLocaleString()}
                  />
                  <Legend
                    wrapperStyle={LEGEND_STYLE}
                    formatter={(value: string) => legendFormatter(value === 'totalBytes' ? 'Bytes' : 'Packets')}
                  />
                  <Area type="monotone" dataKey="totalBytes" stroke={BYTES_COLOR} fill="url(#gradBytes)" strokeWidth={2} {...anim} />
                  <Area type="monotone" dataKey="totalPackets" stroke={PACKETS_COLOR} fill="url(#gradPackets)" strokeWidth={2} {...anim} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
            <SankeyDiagram flows={flows.map((f) => ({ src: f.srcIp, dst: f.dstIp, bytes: f.bytes, protocol: f.protocol }))} />
            <div className="bg-surface-1 rounded-lg p-5 border border-line">
              <h3 className="text-sm font-semibold text-fg mb-4">Top sources</h3>
              {topSources.length === 0 ? (
                <p className="text-xs text-fg-muted text-center py-8">No data</p>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={topSources} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                    <XAxis type="number" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatBytes(v)} />
                    <YAxis type="category" dataKey="ip" tick={CATEGORY_TICK} tickLine={false} axisLine={false} width={110} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} cursor={{ fill: 'var(--color-surface-2)' }} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                    <Bar dataKey="count" fill={SERIES_COLORS[0]} radius={[0, 4, 4, 0]} barSize={16} {...anim} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <div className="bg-surface-1 rounded-lg p-5 border border-line">
              <h3 className="text-sm font-semibold text-fg mb-4">Top destinations</h3>
              {topDestinations.length === 0 ? (
                <p className="text-xs text-fg-muted text-center py-8">No data</p>
              ) : (
                <ResponsiveContainer width="100%" height={280}>
                  <BarChart data={topDestinations} layout="vertical" margin={{ top: 0, right: 8, left: 0, bottom: 0 }}>
                    <XAxis type="number" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v: number) => formatBytes(v)} />
                    <YAxis type="category" dataKey="ip" tick={CATEGORY_TICK} tickLine={false} axisLine={false} width={110} />
                    <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} cursor={{ fill: 'var(--color-surface-2)' }} formatter={(v: unknown) => [formatBytes(Number(v ?? 0)), 'Bytes']} />
                    <Bar dataKey="count" fill={SERIES_COLORS[0]} radius={[0, 4, 4, 0]} barSize={16} {...anim} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </div>

            <Treemap title="Protocol bandwidth" items={protocols.map((p, i) => ({ label: p.protocolName, value: p.bytes, color: protocolColor(p.protocolName, i), detail: p.bytesFormatted }))} />
            <div className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
              <h3 className="text-sm font-semibold text-fg mb-4">Protocol distribution</h3>
              {protocols.length === 0 ? (
                <p className="text-xs text-fg-muted text-center py-8 my-auto">No data</p>
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
                        paddingAngle={2}
                        dataKey="bytes"
                        nameKey="protocolName"
                        labelLine={false}
                        {...anim}
                      >
                        {protocols.map((entry, i) => (
                          <Cell key={entry.protocolName} fill={protocolColor(entry.protocolName, i)} stroke="var(--color-surface-1)" strokeWidth={2} />
                        ))}
                      </Pie>
                      <Tooltip
                        contentStyle={TOOLTIP_STYLE}
                        labelStyle={TOOLTIP_LABEL_STYLE}
                        itemStyle={TOOLTIP_ITEM_STYLE}
                        formatter={(v: unknown, name: unknown) => [formatBytes(Number(v ?? 0)), String(name)]}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="flex flex-wrap justify-center gap-x-4 gap-y-1 mt-2">
                    {protocols.map((p, i) => (
                      <div key={p.protocolName} className="flex items-center gap-1.5 text-xs">
                        <span className="w-2.5 h-2.5 rounded-full flex-shrink-0" style={{ background: protocolColor(p.protocolName, i) }} aria-hidden="true" />
                        <span className="text-fg-muted">{p.protocolName}</span>
                        <span className="font-semibold text-fg">{p.percentage}%</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
            <div className="xl:col-span-2 bg-surface-1 rounded-lg p-5 border border-line">
              <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 mb-4">
                <h3 className="text-sm font-semibold text-fg">Flow records</h3>
                <div className="flex gap-2">
                  <input
                    type="search"
                    aria-label="Filter by IP address"
                    placeholder="Filter IP…"
                    value={filterIp}
                    onChange={(e) => setFilterIp(e.target.value)}
                    className="h-8 w-36 rounded-md border border-line bg-inset px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary"
                  />
                  <select
                    aria-label="Filter by protocol"
                    value={filterProto}
                    onChange={(e) => setFilterProto(e.target.value)}
                    className="h-8 rounded-md border border-line bg-inset px-2 text-sm text-fg outline-none focus:border-primary"
                  >
                    <option value="">All protocols</option>
                    {['TCP', 'UDP', 'ICMP', 'GRE', 'SCTP'].map((p) => (
                      <option key={p} value={p}>{p}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
                <table className="w-full text-left border-collapse">
                  <thead className="sticky top-0 z-sticky bg-surface-1">
                    <tr className="text-xs text-fg-muted border-b border-line">
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
                      <tr><td colSpan={6} className="py-12 text-center text-fg-muted">
                        <div className="flex flex-col items-center gap-2">
                          <span className="material-symbols-outlined text-3xl text-fg-subtle" aria-hidden="true">swap_horiz</span>
                          <span className="text-xs">No flow records yet</span>
                        </div>
                      </td></tr>
                    ) : (
                      filteredFlows.map((f) => {
                        return (
                          <tr key={f.id} className="border-b border-line hover:bg-surface-2 transition-colors">
                            <td className="py-2.5 text-fg-muted font-mono">
                              {new Date(f.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                            </td>
                            <td className="py-2.5 font-mono text-fg">{f.srcIp}{f.srcPort ? `:${f.srcPort}` : ''}</td>
                            <td className="py-2.5 font-mono text-fg">{f.dstIp}{f.dstPort ? `:${f.dstPort}` : ''}</td>
                            <td className="py-2.5">
                              <ProtoChip proto={f.protocol} />
                            </td>
                            <td className="py-2.5 text-right font-mono text-fg">{formatBytes(f.bytes)}</td>
                            <td className="py-2.5 text-right font-mono text-fg-muted">{formatNumber(f.packets)}</td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="bg-surface-1 rounded-lg p-5 border border-line">
              <h3 className="text-sm font-semibold text-fg mb-4">Live flow feed</h3>
              <div className="space-y-2 max-h-[400px] overflow-y-auto">
                {flowFeed.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12">
                    <span className="material-symbols-outlined text-3xl mb-2 text-fg-subtle" aria-hidden="true">rss_feed</span>
                    <p className="text-sm text-fg-muted">Waiting for flow data…</p>
                  </div>
                ) : (
                  flowFeed.map((f, i) => (
                    <div key={i} className="flex items-center gap-3 p-2.5 rounded-md bg-surface-2 border border-line text-xs">
                      <span className="text-xs text-fg-muted font-mono w-14 flex-shrink-0">{f.time}</span>
                      <span className="font-mono text-fg truncate flex-1">{f.src}</span>
                      <span className="material-symbols-outlined text-xs text-fg-muted" aria-label="to" role="img">arrow_forward</span>
                      <span className="font-mono text-fg truncate flex-1">{f.dst}</span>
                      <ProtoChip proto={f.proto} />
                      <span className="font-mono text-fg-muted text-right w-16 flex-shrink-0">{formatBytes(f.bytes)}</span>
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
