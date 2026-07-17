import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { getDevices, getLatestMetrics } from '../api/client';
import { listPhase2 } from '../api/phase2';
import { useSocket } from '../hooks/useSocket';
import type { Device, Metric } from '../api/types';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import DeviceModal from '../components/DeviceModal';
import { useAsyncEffect } from '../hooks/useAsyncEffect';

const COLOR: Record<string, string> = {
  up: '#9bc86e', ok: '#9bc86e',
  warning: '#ebc434', degraded: '#ebc434',
  down: '#e86060', unknown: '#8c8c8c',
};
const STATUS_KEYS = Object.keys(COLOR) as (keyof typeof COLOR)[];
const colorFor = (s: string) => (STATUS_KEYS.includes(s as keyof typeof COLOR) ? COLOR[s as keyof typeof COLOR] : COLOR.unknown);

interface SimNode { id: number; name: string; status: string; ip: string; x: number; y: number; vx: number; vy: number; fx?: number | null; fy?: number | null; r: number; }
interface SimLink { source: SimNode; target: SimNode; }

function statusOf(device: Device, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(device.id)?.status || device.status || 'unknown';
}

export default function NetworkTopology() {
  const fgRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [metricsMap, setMetricsMap] = useState<Map<number, Metric>>(new Map());
  const [locations, setLocations] = useState<Record<string, unknown>[]>([]);
  const [selected, setSelected] = useState<Device | null>(null);
  const [hovered, setHovered] = useState<{ node: SimNode } | null>(null);
  const [filters, setFilters] = useState({ status: 'all', protocol: 'all', location: 'all' });
  const [zoom, setZoom] = useState(1);
  const [size, setSize] = useState({ w: 900, h: 500 });
  const metricsRef = useRef(metricsMap);
  useEffect(() => { metricsRef.current = metricsMap; }, [metricsMap]);

  useAsyncEffect(async (signal) => {
    try {
      const [d, m, l] = await Promise.all([getDevices(), getLatestMetrics(), listPhase2('/locations').catch(() => ({ data: [] }))]);
      if (signal.aborted) return;
      setDevices(d.data || []);
      setMetricsMap(new Map((m.data || []).map((item: Metric) => [item.deviceId, item])));
      setLocations((l.data as Record<string, unknown>[]) || []);
    } catch {
      if (!signal.aborted) { setDevices([]); setLocations([]); }
    }
  }, []);

  useSocket({
    onMetricUpdate: (metric) => {
      const mm = metric as unknown as Metric;
      setMetricsMap((prev) => new Map(prev).set(mm.deviceId, mm));
    },
    onDeviceStatus: (status) => {
      const s = status as { device_id: number; new_status: string };
      setDevices((prev) => prev.map((d) => (d.id === s.device_id ? { ...d, status: s.new_status as Device['status'] } : d)));
    },
  });

  const nodeById = useMemo(() => new Map(devices.map((d) => [d.id, d])), [devices]);
  const protocols = useMemo(() => Array.from(new Set(devices.map((d) => d.protocol))).sort(), [devices]);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const cr = entries[0]?.contentRect;
      if (cr) setSize({ w: Math.max(280, Math.floor(cr.width)), h: 500 });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Visible nodes/links
  const visible = useMemo(() => {
    const list = devices.filter((d) => {
      const status = statusOf(d, metricsMap);
      if (filters.status !== 'all') {
        const ok = filters.status === 'up' ? status === 'up' || status === 'ok' : filters.status === 'down' ? status === 'down' : (status === 'warning' || status === 'degraded');
        if (!ok) return false;
      }
      if (filters.protocol !== 'all' && d.protocol !== filters.protocol) return false;
      if (filters.location !== 'all' && String(d.locationId) !== filters.location) return false;
      return true;
    });
    const ids = new Set(list.map((d) => d.id));
    return { list, ids };
  }, [devices, metricsMap, filters]);

  const linksCount = useMemo(() => visible.list.filter((d) => d.parentDeviceId && visible.ids.has(d.parentDeviceId)).length, [visible]);

  // Simulation
  const sim = useRef<{ nodes: SimNode[]; links: SimLink[]; raf: number; dragging: SimNode | null; mx: number; my: number; pan: { x: number; y: number } }>({ nodes: [], links: [], raf: 0, dragging: null, mx: 0, my: 0, pan: { x: 0, y: 0 } });

  useEffect(() => {
    const W = size.w, H = size.h;
    const prev = sim.current.nodes;
    const prevMap = new Map(prev.map((n) => [n.id, n]));
    const nodes: SimNode[] = visible.list.map((d, i) => {
      const ex = prevMap.get(d.id);
      const angle = (Math.PI * 2 * i) / Math.max(visible.list.length, 1);
      return ex ?? {
        id: d.id, name: d.name, status: statusOf(d, metricsMap), ip: d.ipAddress,
        x: W / 2 + Math.cos(angle) * 160, y: H / 2 + Math.sin(angle) * 160, vx: 0, vy: 0, r: 6,
      };
    });
    const nodeMap = new Map(nodes.map((n) => [n.id, n]));
    const links: SimLink[] = [];
    // 1. Explicit dependency edges from parentDeviceId (real relationships).
    for (const d of visible.list) {
      if (d.parentDeviceId && nodeMap.has(d.parentDeviceId)) {
        links.push({ source: nodeMap.get(d.parentDeviceId)!, target: nodeMap.get(d.id)! });
      }
    }
    // 2. Fallback: build a hub-and-spoke topology from the available data so
    //    the map is never empty. Devices without a parent link to the gateway/
    //    router (by category or name), or to the first device as a central hub.
    if (links.length === 0 && visible.list.length > 1) {
      const isHub = (d: Device) =>
        /gateway|router|core|switch|firewall/i.test(d.deviceCategory || '') ||
        /gateway|router|core|firewall/i.test(d.name || '');
      const hub = visible.list.find(isHub) ?? visible.list[0];
      const hubNode = nodeMap.get(hub.id)!;
      for (const d of visible.list) {
        if (d.id !== hub.id && nodeMap.has(hub.id)) {
          links.push({ source: hubNode, target: nodeMap.get(d.id)! });
        }
      }
    }
    sim.current.nodes = nodes;
    sim.current.links = links;
  }, [visible, metricsMap, size]);

  // Update status colors live (done in animation tick via metricsRef)

  useEffect(() => {
    const canvas = fgRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = size.w * dpr;
    canvas.height = size.h * dpr;
    canvas.style.width = `${size.w}px`;
    canvas.style.height = `${size.h}px`;
    ctx.scale(dpr, dpr);
    const state = sim.current;

    let last = performance.now();
    const tick = (now: number) => {
      const dt = Math.min(32, now - last); last = now;
      const W = size.w, H = size.h;
      const nodes = state.nodes;
      // live status colors
      for (const n of nodes) {
        const dev = nodeById.get(n.id);
        if (dev) n.status = statusOf(dev, metricsRef.current);
      }
      // forces
      for (let i = 0; i < nodes.length; i++) {
        const a = nodes[i];
        if (a.fx != null) { a.x = a.fx; a.y = a.fy!; a.vx = 0; a.vy = 0; continue; }
        // repulsion
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j];
          let dx = a.x - b.x, dy = a.y - b.y;
          let dist = Math.hypot(dx, dy) || 0.01;
          if (dist < 1) { dx = Math.random(); dy = Math.random(); dist = 1; }
          const force = 1400 / (dist * dist);
          const fx = (dx / dist) * force, fy = (dy / dist) * force;
          a.vx += fx; a.vy += fy; b.vx -= fx; b.vy -= fy;
        }
        // centering
        a.vx += (W / 2 - a.x) * 0.0009;
        a.vy += (H / 2 - a.y) * 0.0009;
      }
      // link springs
      for (const l of state.links) {
        const dx = l.target.x - l.source.x, dy = l.target.y - l.source.y;
        const dist = Math.hypot(dx, dy) || 0.01;
        const target = 90;
        const k = (dist - target) * 0.01;
        const fx = (dx / dist) * k, fy = (dy / dist) * k;
        if (l.source.fx == null) { l.source.vx += fx; l.source.vy += fy; }
        if (l.target.fx == null) { l.target.vx -= fx; l.target.vy -= fy; }
      }
      // integrate
      for (const n of nodes) {
        if (n.fx != null) continue;
        n.vx *= 0.85; n.vy *= 0.85;
        n.x += n.vx * (dt / 16); n.y += n.vy * (dt / 16);
        n.x = Math.max(n.r + 4, Math.min(W - n.r - 4, n.x));
        n.y = Math.max(n.r + 4, Math.min(H - n.r - 4, n.y));
      }
      // draw
      ctx.clearRect(0, 0, W, H);
      ctx.save();
      ctx.translate(state.pan.x, state.pan.y);
      ctx.scale(zoom, zoom);
      // links + particles
      const t = now / 600;
      for (const l of state.links) {
        ctx.beginPath();
        ctx.moveTo(l.source.x, l.source.y);
        ctx.lineTo(l.target.x, l.target.y);
        ctx.strokeStyle = 'rgba(150,150,140,0.35)';
        ctx.lineWidth = 1;
        ctx.stroke();
        const p = (t + (l.source.id + l.target.id) * 0.13) % 1;
        const px = l.source.x + (l.target.x - l.source.x) * p;
        const py = l.source.y + (l.target.y - l.source.y) * p;
        ctx.beginPath();
        ctx.arc(px, py, 2, 0, 2 * Math.PI);
        ctx.fillStyle = 'rgba(155,200,110,0.9)';
        ctx.fill();
      }
      // nodes
      for (const n of nodes) {
        const c = colorFor(n.status);
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.r + 2.5, 0, 2 * Math.PI);
        ctx.strokeStyle = c; ctx.globalAlpha = 0.4; ctx.lineWidth = 1; ctx.stroke();
        ctx.globalAlpha = 1;
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.r, 0, 2 * Math.PI);
        ctx.fillStyle = 'rgba(38,38,29,1)'; ctx.fill();
        ctx.strokeStyle = c; ctx.lineWidth = 1.6; ctx.stroke();
        const label = n.name.length > 16 ? `${n.name.slice(0, 15)}…` : n.name;
        ctx.font = '3.2px "JetBrains Mono", monospace';
        ctx.textAlign = 'center'; ctx.textBaseline = 'top';
        ctx.fillStyle = 'rgba(225,225,215,1)';
        ctx.fillText(label, n.x, n.y + n.r + 1.5);
      }
      ctx.restore();
      state.raf = requestAnimationFrame(tick);
    };
    state.raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(state.raf);
  }, [size, zoom, nodeById]);

  const toWorld = (clientX: number, clientY: number) => {
    const rect = fgRef.current!.getBoundingClientRect();
    const x = (clientX - rect.left - sim.current.pan.x) / zoom;
    const y = (clientY - rect.top - sim.current.pan.y) / zoom;
    return { x, y };
  };
  const pick = (x: number, y: number) => {
    for (const n of sim.current.nodes) {
      if (Math.hypot(n.x - x, n.y - y) <= n.r + 3) return n;
    }
    return null;
  };

  const onDown = (e: React.PointerEvent) => {
    const { x, y } = toWorld(e.clientX, e.clientY);
    const n = pick(x, y);
    if (n) { sim.current.dragging = n; n.fx = n.x; n.fy = n.y; }
    else { sim.current.dragging = { x: e.clientX, y: e.clientY } as never; }
  };
  const onMove = (e: React.PointerEvent) => {
    const d = sim.current.dragging;
    if (!d) {
      const { x, y } = toWorld(e.clientX, e.clientY);
      const n = pick(x, y);
      setHovered(n ? { node: n } : null);
      return;
    }
    const { x, y } = toWorld(e.clientX, e.clientY);
    if ('id' in d) { d.fx = x; d.fy = y; d.x = x; d.y = y; }
    else { sim.current.pan.x += e.clientX - (d as unknown as { x: number }).x; sim.current.pan.y += e.clientY - (d as unknown as { y: number }).y; (d as unknown as { x: number }).x = e.clientX; (d as unknown as { y: number }).y = e.clientY; }
  };
  const onUp = () => {
    const d = sim.current.dragging;
    if (d && 'id' in d) { d.fx = null; d.fy = null; }
    sim.current.dragging = null;
  };
  const onClick = (e: React.MouseEvent) => {
    const { x, y } = toWorld(e.clientX, e.clientY);
    const n = pick(x, y);
    if (n) { const dev = nodeById.get(n.id); if (dev) setSelected(dev); }
  };

  const zoomBy = (factor: number) => setZoom((z) => Math.max(0.4, Math.min(3, +(z * factor).toFixed(2))));

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Network Topology"
        subtitle="Live force-directed dependency map built from monitored device relationships and observed status."
        action={<Link to="/devices" className="text-xs text-primary font-semibold hover:underline">Back to devices</Link>}
      />

      <div className="flex flex-wrap items-center gap-3">
        <select value={filters.status} onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))} className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All statuses</option>
          <option value="up">Up</option>
          <option value="warning">Warning</option>
          <option value="down">Down</option>
        </select>
        <select value={filters.protocol} onChange={(e) => setFilters((f) => ({ ...f, protocol: e.target.value }))} className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All protocols</option>
          {protocols.map((p) => <option key={p} value={p}>{p.toUpperCase()}</option>)}
        </select>
        <select value={filters.location} onChange={(e) => setFilters((f) => ({ ...f, location: e.target.value }))} className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All locations</option>
          {locations.map((l) => <option key={String(l.id)} value={String(l.id)}>{String(l.name)}</option>)}
        </select>
        <div className="ml-auto flex items-center gap-2">
          <button onClick={() => zoomBy(1.2)} className="w-8 h-8 grid place-items-center rounded-md border border-outline-variant/30 text-on-surface-variant hover:text-on-surface hover:border-primary/50"><span className="material-symbols-outlined text-base">add</span></button>
          <button onClick={() => zoomBy(0.83)} className="w-8 h-8 grid place-items-center rounded-md border border-outline-variant/30 text-on-surface-variant hover:text-on-surface hover:border-primary/50"><span className="material-symbols-outlined text-base">remove</span></button>
          <button onClick={() => { setZoom(1); sim.current.pan = { x: 0, y: 0 }; }} className="px-3 h-8 rounded-md border border-outline-variant/30 text-xs text-on-surface-variant hover:text-on-surface hover:border-primary/50">Reset</button>
        </div>
        <div className="flex flex-wrap gap-4 text-[10px] uppercase tracking-wide text-on-surface-variant">
          <span className="text-success">● Up</span>
          <span className="text-warning">● Warning</span>
          <span className="text-error">● Down</span>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1fr_280px] gap-5">
        <Card variant="low" className="p-4 overflow-hidden min-h-[560px] relative">
          <div className="text-[10px] uppercase tracking-wide text-on-surface-variant mb-3 flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-success status-dot-live" /> Live topology · {visible.list.length} nodes · {linksCount} edges
          </div>
          <div ref={wrapRef} className="relative">
            {visible.list.length ? (
              <canvas
                ref={fgRef}
                className="touch-none cursor-grab active:cursor-grabbing rounded"
                style={{ width: size.w, height: size.h }}
                onPointerDown={onDown}
                onPointerMove={onMove}
                onPointerUp={onUp}
                onPointerLeave={onUp}
                onClick={onClick}
              />
            ) : (
              <div className="h-[500px] grid place-items-center text-sm text-on-surface-variant">No devices available for topology mapping.</div>
            )}
            {hovered && (
              <div className="absolute top-2 left-2 z-10 bg-surface-container-high border border-outline-variant/30 rounded-lg p-3 text-xs max-w-[220px] pointer-events-none">
                <strong className="font-headline text-sm block truncate">{hovered.node.name}</strong>
                <span className="text-on-surface-variant font-data">{hovered.node.ip}</span>
                <div className="mt-1 flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full" style={{ background: colorFor(hovered.node.status) }} />
                  <span className="uppercase tracking-wide" style={{ color: colorFor(hovered.node.status) }}>{hovered.node.status}</span>
                </div>
              </div>
            )}
          </div>
        </Card>

        <Card variant="low" className="p-5">
          {selected ? (
            <>
              <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Selected node</p>
              <h2 className="font-headline text-xl font-semibold mt-1">{selected.name}</h2>
              <p className="font-data text-xs text-on-surface-variant mt-1">{selected.ipAddress}{selected.port > 0 ? `:${selected.port}` : ''}</p>
              <dl className="mt-6 space-y-4 text-sm">
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Status</dt>
                  <dd style={{ color: colorFor(metricsMap.get(selected.id)?.status || selected.status) }} className="font-semibold capitalize">{metricsMap.get(selected.id)?.status || selected.status}</dd>
                </div>
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Protocol</dt>
                  <dd className="font-semibold uppercase">{selected.protocol}</dd>
                </div>
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Parent dependency</dt>
                  <dd>{selected.parentDeviceId ? (nodeById.get(selected.parentDeviceId)?.name || `Device #${selected.parentDeviceId}`) : 'Auto-discovered edge'}</dd>
                </div>
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Response</dt>
                  <dd>{metricsMap.get(selected.id)?.responseTime ?? '—'}ms</dd>
                </div>
              </dl>
              <button onClick={() => setSelected(null)} className="mt-6 text-xs text-primary font-semibold hover:underline">Clear selection</button>
            </>
          ) : (
            <div className="text-sm text-on-surface-variant pt-12">Select a node to inspect its live status and dependency.</div>
          )}
        </Card>
      </div>

      {selected && (
        <DeviceModal
          device={selected}
          onClose={() => setSelected(null)}
          onDeleted={() => {
            const id = selected.id;
            setSelected(null);
            setDevices((prev) => prev.filter((d) => d.id !== id));
          }}
        />
      )}
    </div>
  );
}
