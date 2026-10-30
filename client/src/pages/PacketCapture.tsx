import { useState, useEffect, useMemo, useRef } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '../api/queryKeys';
import {
  getInterfaces, startCaptureSession, stopCaptureSession,
  getCapturePackets, getCaptureSessions
} from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { CaptureSession, NetworkInterface } from '../api/types';
import type { CapturePacketItem } from '../api/wsEvents';

/** Live packets arrive without a DB id; packets loaded over REST carry one. */
type PacketRow = CapturePacketItem & { id?: number };
import { formatBytes } from '../utils/formatters';
import SectionHeader from '../components/ui/SectionHeader';
import EmptyState from '../components/ui/EmptyState';
import Button from '../components/ui/Button';
import { StatusPill } from '../components/ui/Status';
import { STATUS_META, type StatusMeta } from '../utils/status';

/** Protocol identity marks use the categorical palette; their text stays in text tokens. */
const PROTO_MARK: Record<string, string> = {
  TCP: 'var(--color-chart-1)',
  UDP: 'var(--color-chart-2)',
  ICMP: 'var(--color-chart-3)',
};

function protoMark(proto: string): string {
  return PROTO_MARK[proto] ?? 'var(--color-line-strong)';
}

function ProtoChip({ proto }: { proto: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded bg-surface-3 px-1.5 py-0.5 text-xs font-semibold text-fg">
      <i className="inline-block h-2 w-2 rounded-sm" style={{ background: protoMark(proto) }} aria-hidden="true" />
      {proto}
    </span>
  );
}

function captureErrorMessage(reason: unknown, fallback: string) {
  const body = (reason as { response?: { data?: { error?: { message?: unknown } } } })?.response?.data;
  return typeof body?.error?.message === 'string' ? body.error.message : fallback;
}

const HexDump = ({ hex }: { hex?: string }) => {
  const rows = useMemo(() => {
    if (!hex) return [];
    const bytes = hex.split(' ');
    const result: string[][] = [];
    for (let i = 0; i < bytes.length; i += 16) {
      result.push(bytes.slice(i, i + 16));
    }
    return result;
  }, [hex]);

  if (rows.length === 0) return <p className="text-xs text-fg-muted">No payload data</p>;

  return (
    <div className="font-mono text-xs leading-relaxed overflow-x-auto">
      {rows.map((row, ri) => {
        const offset = (ri * 16).toString(16).padStart(4, '0');
        const hexPart = row.join(' ').padEnd(47, ' ');
        const asciiPart = row.map((b) => {
          const code = parseInt(b, 16);
          return (code >= 32 && code <= 126) ? String.fromCharCode(code) : '.';
        }).join('');

        return (
          <div key={ri} className="flex gap-4 hover:bg-surface-2 px-2 py-0.5 rounded">
            <span className="text-fg-subtle select-none">{offset}</span>
            <span className="text-fg-muted">{hexPart}</span>
            <span className="text-fg-muted border-l border-line pl-4">{asciiPart}</span>
          </div>
        );
      })}
    </div>
  );
};

const VISIBLE_PACKETS = 200;

function sessionMeta(status: string): StatusMeta {
  if (status === 'running') return { ...STATUS_META.info, label: 'Running' };
  if (status === 'error') return { ...STATUS_META.down, label: 'Error' };
  if (status === 'stopped' || status === 'completed') return { ...STATUS_META.unknown, label: status.charAt(0).toUpperCase() + status.slice(1) };
  return { ...STATUS_META.unknown, label: status || 'Unknown' };
}

const EMPTY_INTERFACES: NetworkInterface[] = [];
const EMPTY_SESSIONS: CaptureSession[] = [];

export default function PacketCapture() {
  const queryClient = useQueryClient();
  const [selectedIface, setSelectedIface] = useState('');
  const [bpfFilter, setBpfFilter] = useState('');
  const [activeSession, setActiveSession] = useState<CaptureSession | null>(null);
  const [packets, setPackets] = useState<PacketRow[]>([]);
  const [selectedPacket, setSelectedPacket] = useState<PacketRow | null>(null);
  const [isStarting, setIsStarting] = useState(false);
  const [error, setError] = useState('');
  const [autoScroll, setAutoScroll] = useState(false);
  const tableEndRef = useRef<HTMLDivElement>(null);
  const activeSessionRef = useRef(activeSession);

  useEffect(() => {
    activeSessionRef.current = activeSession;
  }, [activeSession]);

  const interfacesQuery = useQuery({
    queryKey: queryKeys.capture.interfaces(),
    queryFn: ({ signal }) => getInterfaces({ signal }),
  });
  // Invalidated on capture:status events by useLiveQueryBridge.
  const sessionsQuery = useQuery({
    queryKey: queryKeys.capture.sessions(),
    queryFn: ({ signal }) => getCaptureSessions({ signal }),
  });
  const interfaces = interfacesQuery.data?.data || EMPTY_INTERFACES;
  const sessions = sessionsQuery.data?.data || EMPTY_SESSIONS;
  // Default to the first interface until the user picks one.
  const currentIface = selectedIface || interfaces[0]?.name || '';
  const loadSessions = () => queryClient.invalidateQueries({ queryKey: queryKeys.capture.sessions() });

  useEffect(() => {
    if (autoScroll && tableEndRef.current) {
      requestAnimationFrame(() => {
        tableEndRef.current?.scrollIntoView({ behavior: 'smooth' });
      });
    }
  }, [packets.length, autoScroll]);

  const handleStart = async () => {
    if (!currentIface) return;
    setError('');
    setIsStarting(true);
    try {
      const res = await startCaptureSession({
        interface: currentIface,
        filter: bpfFilter || undefined
      });
      setActiveSession(res.data);
      setPackets([]);
      setSelectedPacket(null);
    } catch (err: unknown) {
      setError(captureErrorMessage(err, 'Failed to start capture'));
    } finally {
      setIsStarting(false);
    }
  };

  const handleStop = async () => {
    if (!activeSession) return;
    try {
      await stopCaptureSession(activeSession.id);
      setActiveSession(null);
      void loadSessions();
    } catch (err: unknown) {
      setError(captureErrorMessage(err, 'Failed to stop capture'));
    }
  };

  const loadSessionPackets = async (sessionId: number) => {
    try {
      const res = await getCapturePackets(sessionId, { limit: 500 });
      setPackets(res.data || []);
    } catch { /* handled */ }
  };

  useSocket({
    onPacketCaptured: (batch) => {
      const session = activeSessionRef.current;
      if (session && batch.sessionId === session.id && batch.packets) {
        setPackets((prev) => {
          const next = [...prev, ...batch.packets];
          if (next.length > 1000) next.splice(0, next.length - 1000);
          return next;
        });
      }
    },
    onCaptureStatus: (d) => {
      if (d.status === 'stopped' || d.status === 'error') {
        const session = activeSessionRef.current;
        if (session && d.sessionId === session.id) {
          if (d.error) setError(d.error);
          setActiveSession(null);
        }
      }
    },
  });

  const protoCounts = useMemo(() => packets.reduce<Record<string, number>>((acc, p) => {
    acc[p.protocol] = (acc[p.protocol] || 0) + 1;
    return acc;
  }, {}), [packets]);
  const protoEntries = useMemo(() => Object.entries(protoCounts).sort((a, b) => b[1] - a[1]), [protoCounts]);

  const visiblePackets = useMemo(() => packets.slice(-VISIBLE_PACKETS), [packets]);
  const packetSizes = useMemo(() => {
    const buckets = [{ label: '0–64', min: 0, max: 64 }, { label: '65–128', min: 65, max: 128 }, { label: '129–256', min: 129, max: 256 }, { label: '257–512', min: 257, max: 512 }, { label: '512+', min: 513, max: Infinity }];
    return buckets.map(bucket => ({ ...bucket, count: packets.filter(packet => packet.length >= bucket.min && packet.length <= bucket.max).length }));
  }, [packets]);

  return (
    <div>
      <SectionHeader
        title="Packet capture"
        subtitle="Capture, inspect, and analyze network traffic in real-time."
      />

      <div className="bg-surface-1 rounded-lg p-5 border border-line mb-6">
        <div className="flex flex-col md:flex-row items-start md:items-center gap-4">
          <div className="flex-shrink-0">
            <label htmlFor="capture-iface" className="text-xs font-medium text-fg-muted block mb-1.5">Interface</label>
            <select
              id="capture-iface"
              value={currentIface}
              onChange={(e) => setSelectedIface(e.target.value)}
              disabled={activeSession?.status === 'running'}
              className="h-control bg-inset border border-line rounded-md px-3 text-sm text-fg font-mono outline-none focus:border-primary disabled:opacity-50 min-w-[140px]"
            >
              {interfaces.map((iface) => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} {iface.flags?.includes('up') ? '(up)' : '(down)'}
                </option>
              ))}
            </select>
          </div>

          <div className="flex-1 min-w-0">
            <label htmlFor="capture-bpf" className="text-xs font-medium text-fg-muted block mb-1.5">BPF filter</label>
            <input
              id="capture-bpf"
              type="text"
              value={bpfFilter}
              onChange={(e) => setBpfFilter(e.target.value)}
              disabled={activeSession?.status === 'running'}
              placeholder="e.g. tcp port 80, icmp, host 192.168.1.1"
              className="w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg font-mono placeholder:text-fg-subtle outline-none focus:border-primary disabled:opacity-50"
            />
          </div>

          <div className="flex-shrink-0 self-end">
            {activeSession?.status === 'running' ? (
              <Button variant="danger" icon="stop_circle" onClick={handleStop}>Stop capture</Button>
            ) : (
              <Button icon="play_circle" onClick={handleStart} disabled={isStarting || !currentIface}>
                {isStarting ? 'Starting…' : 'Start capture'}
              </Button>
            )}
          </div>

          {activeSession?.status === 'running' && (
            <div className="flex items-center gap-4 ml-auto">
              <StatusPill status={{ ...STATUS_META.info, label: 'Capturing' }} />
              <div className="text-center">
                <p className="text-xs text-fg-muted">Packets</p>
                <p className="font-semibold text-lg text-fg tabular-nums">{packets.length}</p>
              </div>
            </div>
          )}
        </div>

        {error && (
          <div role="alert" className="mt-3 flex items-center gap-2 p-3 rounded-md border border-line border-l-4 border-l-error bg-inset">
            <span className="material-symbols-outlined text-error text-lg" aria-hidden="true">error</span>
            <span className="text-fg text-sm">{error}</span>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-5 mb-6">
        <div className="bg-surface-1 rounded-lg p-4 border border-line"><p className="text-sm font-medium text-fg-muted">Packets captured</p><p className="mt-2 text-2xl font-semibold tabular-nums">{packets.length.toLocaleString()}</p><p className="text-xs text-fg-muted mt-2">In the current view</p></div>
        <div className="bg-surface-1 rounded-lg p-4 border border-line"><p className="text-sm font-medium text-fg-muted">Packet size (bytes)</p><div className="flex items-end gap-2 h-14 mt-2">{packetSizes.map(bucket => <div key={bucket.label} className="flex-1 flex flex-col justify-end h-full"><i className="bg-chart-1 rounded-t-sm" title={`${bucket.label}: ${bucket.count}`} style={{ height: `${Math.max(5, packets.length ? bucket.count / packets.length * 100 : 5)}%` }} /><span className="text-xs text-fg-muted mt-1 text-center">{bucket.label}</span></div>)}</div></div>
        <div className="bg-surface-1 rounded-lg p-4 border border-line"><p className="text-sm font-medium text-fg-muted">Session</p><div className="mt-2">{activeSession ? <StatusPill status={sessionMeta(activeSession.status)} size="md" /> : <span className="text-sm text-fg-muted">No session</span>}</div><p className="text-xs text-fg-muted mt-2">{activeSession?.status === 'running' ? 'Capturing packets in real time.' : 'Start a capture to activate the stream.'}</p></div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        <div className="xl:col-span-2 bg-surface-1 rounded-lg border border-line flex flex-col" style={{ maxHeight: '600px' }}>
          <div className="flex items-center justify-between p-4 border-b border-line">
            <h3 className="text-sm font-semibold flex items-center gap-2">
              <span className="material-symbols-outlined text-fg-muted text-lg" aria-hidden="true">list_alt</span>
              Captured packets
              {packets.length > 0 && (
                <span className="text-fg-muted font-mono text-xs ml-2">({packets.length})</span>
              )}
            </h3>
            <button
              type="button"
              aria-pressed={autoScroll}
              onClick={() => setAutoScroll(!autoScroll)}
              className={`text-xs flex items-center gap-1 px-2 py-1 rounded ${autoScroll ? 'text-fg bg-surface-3' : 'text-fg-muted hover:text-fg'}`}
            >
              <span className="material-symbols-outlined text-sm" aria-hidden="true">{autoScroll ? 'vertical_align_bottom' : 'pause'}</span>
              {autoScroll ? 'Auto-scroll' : 'Paused'}
            </button>
          </div>
          <div className="overflow-auto flex-1">
            <table className="w-full text-left border-collapse">
              <thead className="sticky top-0 bg-surface-2 z-sticky">
                <tr className="text-xs text-fg-muted border-b border-line">
                  <th className="py-2 px-3 font-medium w-12">No.</th>
                  <th className="py-2 px-2 font-medium">Time</th>
                  <th className="py-2 px-2 font-medium">Source</th>
                  <th className="py-2 px-2 font-medium">Destination</th>
                  <th className="py-2 px-2 font-medium">Proto</th>
                  <th className="py-2 px-2 font-medium text-right">Len</th>
                  <th className="py-2 px-2 font-medium">Info</th>
                </tr>
              </thead>
              <tbody className="text-xs font-mono">
                {packets.length === 0 ? (
                  <tr><td colSpan={7} className="py-16">
                    <EmptyState
                      icon="network_check"
                      title={activeSession ? 'Waiting for packets…' : 'Start a capture to see packets'}
                    />
                  </td></tr>
                ) : (
                  visiblePackets.map((pkt, idx) => {
                    const globalIdx = packets.length - VISIBLE_PACKETS + idx;
                    const pktId = pkt.id ?? globalIdx;
                    const isSelected = selectedPacket && (selectedPacket.id ?? packets.indexOf(selectedPacket)) === pktId;
                    return (
                      <tr
                        key={pktId}
                        onClick={() => setSelectedPacket(isSelected ? null : pkt)}
                        aria-selected={!!isSelected}
                        className={`border-b border-line cursor-pointer transition-colors ${
                          isSelected ? 'bg-primary/12' : 'hover:bg-surface-2'
                        }`}
                      >
                        <td className="py-1.5 px-3 text-fg-muted">{pkt.id ?? globalIdx}</td>
                        <td className="py-1.5 px-2 text-fg-muted">
                          {new Date(pkt.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                        </td>
                        <td className="py-1.5 px-2 text-fg">
                          {pkt.srcIp}{pkt.srcPort ? `:${pkt.srcPort}` : ''}
                        </td>
                        <td className="py-1.5 px-2 text-fg">
                          {pkt.dstIp}{pkt.dstPort ? `:${pkt.dstPort}` : ''}
                        </td>
                        <td className="py-1.5 px-2">
                          <ProtoChip proto={pkt.protocol} />
                        </td>
                        <td className="py-1.5 px-2 text-right text-fg-muted">{pkt.length}</td>
                        <td className="py-1.5 px-2 text-fg-muted truncate max-w-[200px]" title={pkt.flags || ''}>{pkt.flags || ''}</td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
            <div ref={tableEndRef} />
          </div>
        </div>

        <div className="flex flex-col gap-6">
          <div className="bg-surface-1 rounded-lg border border-line flex flex-col" style={{ maxHeight: '380px' }}>
            <div className="p-4 border-b border-line">
              <h3 className="text-sm font-semibold flex items-center gap-2">
                <span className="material-symbols-outlined text-fg-muted text-lg" aria-hidden="true">search</span>
                Packet detail
              </h3>
            </div>
            <div className="p-4 overflow-auto flex-1">
              {selectedPacket ? (
                <div className="space-y-4">
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-fg-muted w-14">Proto:</span>
                      <ProtoChip proto={selectedPacket.protocol} />
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-fg-muted w-14">Source:</span>
                      <span className="font-mono text-fg">{selectedPacket.srcIp}:{selectedPacket.srcPort}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-fg-muted w-14">Dest:</span>
                      <span className="font-mono text-fg">{selectedPacket.dstIp}:{selectedPacket.dstPort}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-fg-muted w-14">Length:</span>
                      <span className="font-mono text-fg">{selectedPacket.length} bytes</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-fg-muted w-14">Info:</span>
                      <span className="text-fg text-xs">{selectedPacket.flags || '-'}</span>
                    </div>
                  </div>

                  <div>
                    <p className="text-xs text-fg-muted mb-2">Hex dump</p>
                    <div className="bg-inset rounded-md p-3 border border-line overflow-x-auto">
                      <HexDump hex={selectedPacket.payload} />
                    </div>
                  </div>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center h-full py-8">
                  <span className="material-symbols-outlined text-3xl mb-2 text-fg-subtle" aria-hidden="true">touch_app</span>
                  <p className="text-xs text-fg-muted">Select a packet to inspect</p>
                </div>
              )}
            </div>
          </div>

          <div className="bg-surface-1 rounded-lg p-4 border border-line">
            <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-fg-muted text-lg" aria-hidden="true">donut_small</span>
              Protocol mix
            </h3>
            {protoEntries.length === 0 ? (
              <p className="text-xs text-fg-muted text-center py-4">No packets</p>
            ) : (
              <div className="space-y-2">
                {protoEntries.slice(0, 6).map(([proto, count]) => {
                  const pct = packets.length > 0 ? Math.round((count / packets.length) * 100) : 0;
                  return (
                    <div key={proto}>
                      <div className="flex justify-between text-xs mb-0.5">
                        <span className="font-medium text-fg">{proto}</span>
                        <span className="text-fg-muted tabular-nums">{count} ({pct}%)</span>
                      </div>
                      <div className="h-1.5 bg-inset rounded">
                        <div className="h-1.5 rounded transition-[width] duration-[var(--duration-normal)]" style={{ width: `${pct}%`, background: protoMark(proto) }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="bg-surface-1 rounded-lg p-5 border border-line">
        <h3 className="text-sm font-semibold mb-4 flex items-center gap-2">
          <span className="material-symbols-outlined text-fg-muted text-lg" aria-hidden="true">history</span>
          Capture history
        </h3>
        {sessions.length === 0 ? (
          <p className="text-xs text-fg-muted text-center py-6">No capture sessions yet</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-line">
                  <th className="pb-2 font-medium">ID</th>
                  <th className="pb-2 font-medium">Interface</th>
                  <th className="pb-2 font-medium">Filter</th>
                  <th className="pb-2 font-medium">Status</th>
                  <th className="pb-2 font-medium text-right">Packets</th>
                  <th className="pb-2 font-medium text-right">Bytes</th>
                  <th className="pb-2 font-medium">Started</th>
                  <th className="pb-2 font-medium"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody className="text-xs">
                {sessions.map((s) => {
                  const isRunning = s.status === 'running';
                  return (
                    <tr key={s.id} className="border-b border-line hover:bg-surface-2 transition-colors">
                      <td className="py-2.5 font-mono text-fg-muted">#{s.id}</td>
                      <td className="py-2.5 font-mono text-fg">{s.interfaceName}</td>
                      <td className="py-2.5 text-fg-muted font-mono">{s.filter || '—'}</td>
                      <td className="py-2.5">
                        <StatusPill status={sessionMeta(s.status)} />
                      </td>
                      <td className="py-2.5 text-right font-mono text-fg">{s.totalPackets}</td>
                      <td className="py-2.5 text-right font-mono text-fg-muted">{formatBytes(s.totalBytes)}</td>
                      <td className="py-2.5 text-fg-muted font-mono">
                        {new Date(s.startedAt).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </td>
                      <td className="py-2.5">
                        {!isRunning && s.totalPackets > 0 && (
                          <Button size="sm" variant="ghost" icon="visibility" onClick={() => { setActiveSession(s); void loadSessionPackets(s.id); }} aria-label={`View packets from session ${s.id}`}>
                            View packets
                          </Button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
