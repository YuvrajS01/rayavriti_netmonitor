import { useState, useEffect, useCallback, useRef } from 'react';
import {
  getInterfaces, startCaptureSession, stopCaptureSession,
  getCapturePackets, getCaptureSessions
} from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { CapturedPacket, CaptureSession, NetworkInterface } from '../api/types';

const PROTO_COLORS: Record<string, { text: string; bg: string; border: string }> = {
  TCP: { text: '#6ee7f7', bg: 'rgba(110,231,247,0.08)', border: 'rgba(110,231,247,0.2)' },
  UDP: { text: '#d9fd3a', bg: 'rgba(217,253,58,0.08)', border: 'rgba(217,253,58,0.2)' },
  ICMP: { text: '#f59e0b', bg: 'rgba(245,158,11,0.08)', border: 'rgba(245,158,11,0.2)' },
  UNKNOWN: { text: '#8a8a78', bg: 'rgba(138,138,120,0.06)', border: 'rgba(138,138,120,0.15)' },
};

function getProtoStyle(proto: string) {
  return PROTO_COLORS[proto] || PROTO_COLORS.UNKNOWN;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function HexDump({ hex }: { hex: string }) {
  if (!hex) return <p className="text-xs text-on-surface-variant opacity-50">No payload data</p>;

  const bytes = hex.split(' ');
  const rows: string[][] = [];
  for (let i = 0; i < bytes.length; i += 16) {
    rows.push(bytes.slice(i, i + 16));
  }

  return (
    <div className="font-mono text-[11px] leading-relaxed overflow-x-auto">
      {rows.map((row, ri) => {
        const offset = (ri * 16).toString(16).padStart(4, '0');
        const hexPart = row.join(' ').padEnd(47, ' ');
        const asciiPart = row.map((b) => {
          const code = parseInt(b, 16);
          return (code >= 32 && code <= 126) ? String.fromCharCode(code) : '.';
        }).join('');

        return (
          <div key={ri} className="flex gap-4 hover:bg-surface-container-highest/30 px-2 py-0.5 rounded">
            <span className="text-primary/60 select-none">{offset}</span>
            <span className="text-[#6ee7f7]">{hexPart}</span>
            <span className="text-on-surface-variant border-l border-outline-variant/20 pl-4">{asciiPart}</span>
          </div>
        );
      })}
    </div>
  );
}

export default function PacketCapture() {
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [selectedIface, setSelectedIface] = useState('');
  const [bpfFilter, setBpfFilter] = useState('');
  const [activeSession, setActiveSession] = useState<CaptureSession | null>(null);
  const [packets, setPackets] = useState<CapturedPacket[]>([]);
  const [selectedPacket, setSelectedPacket] = useState<CapturedPacket | null>(null);
  const [sessions, setSessions] = useState<CaptureSession[]>([]);
  const [isStarting, setIsStarting] = useState(false);
  const [error, setError] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const tableEndRef = useRef<HTMLDivElement>(null);

  const loadInterfaces = useCallback(async () => {
    try {
      const res = await getInterfaces();
      setInterfaces(res.data || []);
      if (res.data?.length > 0 && !selectedIface) {
        setSelectedIface(res.data[0].name);
      }
    } catch { /* handled by interceptor */ }
  }, [selectedIface]);

  const loadSessions = useCallback(async () => {
    try {
      const res = await getCaptureSessions();
      setSessions(res.data || []);
    } catch { /* handled */ }
  }, []);

  useEffect(() => { loadInterfaces(); loadSessions(); }, [loadInterfaces, loadSessions]);

  useEffect(() => {
    if (autoScroll && tableEndRef.current) {
      tableEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [packets.length, autoScroll]);

  const handleStart = async () => {
    if (!selectedIface) return;
    setError('');
    setIsStarting(true);
    try {
      const res = await startCaptureSession({
        interface: selectedIface,
        filter: bpfFilter || undefined
      });
      setActiveSession(res.data);
      setPackets([]);
      setSelectedPacket(null);
    } catch (err: any) {
      setError(err.response?.data?.error?.message || err.message || 'Failed to start capture');
    } finally {
      setIsStarting(false);
    }
  };

  const handleStop = async () => {
    if (!activeSession) return;
    try {
      await stopCaptureSession(activeSession.id);
      setActiveSession(null);
      loadSessions();
    } catch (err: any) {
      setError(err.response?.data?.error?.message || 'Failed to stop capture');
    }
  };

  const loadSessionPackets = async (sessionId: number) => {
    try {
      const res = await getCapturePackets(sessionId, { limit: 500 });
      setPackets(res.data || []);
    } catch { /* handled */ }
  };

  useSocket({
    onPacketCaptured: (data) => {
      const pkt = data as unknown as CapturedPacket;
      if (activeSession && pkt.sessionId === activeSession.id) {
        setPackets((prev) => {
          const next = [...prev, pkt];
          if (next.length > 1000) next.shift();
          return next;
        });
      }
    },
    onCaptureStatus: (data) => {
      const d = data as { sessionId: number; status: string; packetCount?: number; bytesCaptured?: number };
      if (d.status === 'stopped' || d.status === 'error') {
        if (activeSession && d.sessionId === activeSession.id) {
          setActiveSession(null);
          loadSessions();
        }
      }
    },
  });

  // Protocol distribution from captured packets
  const protoCounts = packets.reduce<Record<string, number>>((acc, p) => {
    acc[p.protocol] = (acc[p.protocol] || 0) + 1;
    return acc;
  }, {});
  const protoEntries = Object.entries(protoCounts).sort((a, b) => b[1] - a[1]);

  return (
    <div>
      {/* Header */}
      <header className="mb-8">
        <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">
          Packet Capture
        </h1>
        <p className="text-on-surface-variant font-body max-w-xl">
          Live packet sniffing and analysis. Capture, inspect, and analyze network traffic in real-time.
        </p>
      </header>

      {/* Capture Controls */}
      <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 mb-6">
        <div className="flex flex-col md:flex-row items-start md:items-center gap-4">
          {/* Interface Select */}
          <div className="flex-shrink-0">
            <label className="text-[10px] uppercase tracking-widest text-on-surface-variant block mb-1">Interface</label>
            <select
              value={selectedIface}
              onChange={(e) => setSelectedIface(e.target.value)}
              disabled={!!activeSession}
              className="bg-surface-container-highest border border-outline-variant/30 rounded-lg px-4 py-2.5 text-sm text-on-surface font-mono focus:outline-none focus:border-primary/50 disabled:opacity-50 min-w-[140px]"
            >
              {interfaces.map((iface) => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} {iface.flags.includes('up') ? '●' : '○'}
                </option>
              ))}
            </select>
          </div>

          {/* BPF Filter */}
          <div className="flex-1 min-w-0">
            <label className="text-[10px] uppercase tracking-widest text-on-surface-variant block mb-1">BPF Filter</label>
            <input
              type="text"
              value={bpfFilter}
              onChange={(e) => setBpfFilter(e.target.value)}
              disabled={!!activeSession}
              placeholder="e.g. tcp port 80, icmp, host 192.168.1.1"
              className="w-full bg-surface-container-highest border border-outline-variant/30 rounded-lg px-4 py-2.5 text-sm text-on-surface font-mono placeholder:text-on-surface-variant/40 focus:outline-none focus:border-primary/50 disabled:opacity-50"
            />
          </div>

          {/* Start/Stop Button */}
          <div className="flex-shrink-0 self-end">
            {activeSession ? (
              <button
                onClick={handleStop}
                className="flex items-center gap-2 bg-error/20 text-error border border-error/30 px-6 py-2.5 rounded-lg font-headline font-bold text-xs uppercase tracking-widest hover:bg-error hover:text-on-error transition-all"
              >
                <span className="material-symbols-outlined text-lg">stop_circle</span>
                Stop Capture
              </button>
            ) : (
              <button
                onClick={handleStart}
                disabled={isStarting || !selectedIface}
                className="flex items-center gap-2 bg-primary/20 text-primary border border-primary/30 px-6 py-2.5 rounded-lg font-headline font-bold text-xs uppercase tracking-widest hover:bg-primary hover:text-[#0e0e09] transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <span className="material-symbols-outlined text-lg">play_circle</span>
                {isStarting ? 'Starting...' : 'Start Capture'}
              </button>
            )}
          </div>

          {/* Live Stats */}
          {activeSession && (
            <div className="flex items-center gap-4 ml-auto">
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-error animate-pulse" />
                <span className="text-error font-mono text-xs uppercase tracking-wider">Live</span>
              </div>
              <div className="text-center">
                <p className="text-[10px] text-on-surface-variant uppercase tracking-widest">Packets</p>
                <p className="font-headline font-bold text-lg text-on-surface">{packets.length}</p>
              </div>
            </div>
          )}
        </div>

        {error && (
          <div className="mt-3 flex items-center gap-2 p-3 rounded-lg bg-error/10 border border-error/20">
            <span className="material-symbols-outlined text-error text-lg">error</span>
            <span className="text-error text-xs">{error}</span>
          </div>
        )}
      </div>

      {/* Main Content: Packet Table + Detail Panel */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        {/* Packet Table */}
        <div className="xl:col-span-2 bg-surface-container-high rounded-xl border border-outline-variant/20 flex flex-col" style={{ maxHeight: '600px' }}>
          <div className="flex items-center justify-between p-4 border-b border-outline-variant/20">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest flex items-center gap-2">
              <span className="material-symbols-outlined text-primary text-lg">list_alt</span>
              Captured Packets
              {packets.length > 0 && (
                <span className="text-on-surface-variant font-mono text-[10px] ml-2">({packets.length})</span>
              )}
            </h3>
            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className={`text-xs flex items-center gap-1 px-2 py-1 rounded ${autoScroll ? 'text-primary bg-primary/10' : 'text-on-surface-variant'}`}
            >
              <span className="material-symbols-outlined text-sm">{autoScroll ? 'vertical_align_bottom' : 'pause'}</span>
              {autoScroll ? 'Auto-scroll' : 'Paused'}
            </button>
          </div>
          <div className="overflow-auto flex-1">
            <table className="w-full text-left border-collapse">
              <thead className="sticky top-0 bg-surface-container-high z-10">
                <tr className="text-[9px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
                  <th className="py-2 px-3 font-medium w-12">No.</th>
                  <th className="py-2 px-2 font-medium">Time</th>
                  <th className="py-2 px-2 font-medium">Source</th>
                  <th className="py-2 px-2 font-medium">Destination</th>
                  <th className="py-2 px-2 font-medium">Proto</th>
                  <th className="py-2 px-2 font-medium text-right">Len</th>
                  <th className="py-2 px-2 font-medium">Info</th>
                </tr>
              </thead>
              <tbody className="text-[11px] font-mono">
                {packets.length === 0 ? (
                  <tr><td colSpan={7} className="py-16 text-center text-on-surface-variant opacity-50">
                    <div className="flex flex-col items-center gap-2">
                      <span className="material-symbols-outlined text-4xl">network_check</span>
                      <span className="text-[10px] uppercase tracking-widest font-sans">
                        {activeSession ? 'Waiting for packets...' : 'Start a capture to see packets'}
                      </span>
                    </div>
                  </td></tr>
                ) : (
                  packets.map((pkt) => {
                    const style = getProtoStyle(pkt.protocol);
                    const isSelected = selectedPacket?.id === pkt.id;
                    return (
                      <tr
                        key={pkt.id}
                        onClick={() => setSelectedPacket(isSelected ? null : pkt)}
                        className={`border-b cursor-pointer transition-colors ${
                          isSelected
                            ? 'bg-primary/10 border-primary/20'
                            : 'border-outline-variant/5 hover:bg-surface-container-highest/50'
                        }`}
                        style={{ borderLeftWidth: 2, borderLeftColor: style.text }}
                      >
                        <td className="py-1.5 px-3 text-on-surface-variant">{pkt.id}</td>
                        <td className="py-1.5 px-2 text-on-surface-variant">
                          {new Date(pkt.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', fractionalSecondDigits: 3 } as any)}
                        </td>
                        <td className="py-1.5 px-2 text-on-surface">
                          {pkt.srcIp}{pkt.srcPort ? `:${pkt.srcPort}` : ''}
                        </td>
                        <td className="py-1.5 px-2 text-on-surface">
                          {pkt.dstIp}{pkt.dstPort ? `:${pkt.dstPort}` : ''}
                        </td>
                        <td className="py-1.5 px-2">
                          <span className="inline-block px-1.5 py-0.5 rounded text-[9px] font-bold"
                            style={{ color: style.text, background: style.bg, border: `1px solid ${style.border}` }}>
                            {pkt.protocol}
                          </span>
                        </td>
                        <td className="py-1.5 px-2 text-right text-on-surface-variant">{pkt.length}</td>
                        <td className="py-1.5 px-2 text-on-surface-variant truncate max-w-[200px]" title={pkt.flags || ''}>{pkt.flags || ''}</td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
            <div ref={tableEndRef} />
          </div>
        </div>

        {/* Packet Detail Panel + Protocol Stats */}
        <div className="flex flex-col gap-6">
          {/* Packet Detail */}
          <div className="bg-surface-container-high rounded-xl border border-outline-variant/20 flex flex-col" style={{ maxHeight: '380px' }}>
            <div className="p-4 border-b border-outline-variant/20">
              <h3 className="text-sm font-headline font-bold uppercase tracking-widest flex items-center gap-2">
                <span className="material-symbols-outlined text-[#6ee7f7] text-lg">search</span>
                Packet Detail
              </h3>
            </div>
            <div className="p-4 overflow-auto flex-1">
              {selectedPacket ? (
                <div className="space-y-4">
                  {/* Header info */}
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-on-surface-variant w-14">Proto:</span>
                      <span className="font-bold" style={{ color: getProtoStyle(selectedPacket.protocol).text }}>{selectedPacket.protocol}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-on-surface-variant w-14">Source:</span>
                      <span className="font-mono text-on-surface">{selectedPacket.srcIp}:{selectedPacket.srcPort}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-on-surface-variant w-14">Dest:</span>
                      <span className="font-mono text-on-surface">{selectedPacket.dstIp}:{selectedPacket.dstPort}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-on-surface-variant w-14">Length:</span>
                      <span className="font-mono text-on-surface">{selectedPacket.length} bytes</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-on-surface-variant w-14">Info:</span>
                      <span className="text-on-surface text-[11px]">{selectedPacket.flags || '-'}</span>
                    </div>
                  </div>

                  {/* Hex dump */}
                  <div>
                    <p className="text-[10px] uppercase tracking-widest text-on-surface-variant mb-2">Hex Dump</p>
                    <div className="bg-[#0e0e09] rounded-lg p-3 border border-outline-variant/10 overflow-x-auto">
                      <HexDump hex={selectedPacket.payload} />
                    </div>
                  </div>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center h-full opacity-50 py-8">
                  <span className="material-symbols-outlined text-3xl mb-2 text-on-surface-variant">touch_app</span>
                  <p className="text-[10px] text-on-surface-variant uppercase tracking-widest">Select a packet to inspect</p>
                </div>
              )}
            </div>
          </div>

          {/* Protocol Quick Stats */}
          <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20">
            <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-[#c084fc] text-lg">donut_small</span>
              Protocol Stats
            </h3>
            {protoEntries.length === 0 ? (
              <p className="text-xs text-on-surface-variant opacity-50 text-center py-4">No packets</p>
            ) : (
              <div className="space-y-2">
                {protoEntries.slice(0, 6).map(([proto, count]) => {
                  const pct = packets.length > 0 ? Math.round((count / packets.length) * 100) : 0;
                  const style = getProtoStyle(proto);
                  return (
                    <div key={proto}>
                      <div className="flex justify-between text-xs mb-0.5">
                        <span style={{ color: style.text }}>{proto}</span>
                        <span className="text-on-surface-variant">{count} ({pct}%)</span>
                      </div>
                      <div className="h-1.5 bg-surface-container-highest rounded">
                        <div className="h-1.5 rounded transition-all duration-500" style={{ width: `${pct}%`, background: style.text }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Past Capture Sessions */}
      <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
        <h3 className="text-sm font-headline font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
          <span className="material-symbols-outlined text-on-surface-variant text-lg">history</span>
          Capture History
        </h3>
        {sessions.length === 0 ? (
          <p className="text-xs text-on-surface-variant opacity-50 text-center py-6">No capture sessions yet</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
                  <th className="pb-2 font-medium">ID</th>
                  <th className="pb-2 font-medium">Interface</th>
                  <th className="pb-2 font-medium">Filter</th>
                  <th className="pb-2 font-medium">Status</th>
                  <th className="pb-2 font-medium text-right">Packets</th>
                  <th className="pb-2 font-medium text-right">Bytes</th>
                  <th className="pb-2 font-medium">Started</th>
                  <th className="pb-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="text-xs">
                {sessions.map((s) => {
                  const isRunning = s.status === 'running';
                  const isError = s.status === 'error';
                  return (
                    <tr key={s.id} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-colors">
                      <td className="py-2.5 font-mono text-on-surface-variant">#{s.id}</td>
                      <td className="py-2.5 font-mono text-on-surface">{s.interfaceName}</td>
                      <td className="py-2.5 text-on-surface-variant font-mono">{s.filter || '—'}</td>
                      <td className="py-2.5">
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase ${
                          isRunning ? 'text-primary bg-primary/10 border border-primary/20'
                            : isError ? 'text-error bg-error/10 border border-error/20'
                            : 'text-on-surface-variant bg-surface-container-highest border border-outline-variant/20'
                        }`}>
                          {isRunning && <div className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />}
                          {s.status}
                        </span>
                      </td>
                      <td className="py-2.5 text-right font-mono text-on-surface">{s.totalPackets}</td>
                      <td className="py-2.5 text-right font-mono text-on-surface-variant">{formatBytes(s.totalBytes)}</td>
                      <td className="py-2.5 text-on-surface-variant font-mono">
                        {new Date(s.startedAt).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </td>
                      <td className="py-2.5">
                        {!isRunning && s.totalPackets > 0 && (
                          <button
                            onClick={() => { setActiveSession(s); loadSessionPackets(s.id); }}
                            className="text-primary hover:text-primary/80 text-[10px] uppercase tracking-wider font-bold"
                          >
                            View Packets
                          </button>
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
