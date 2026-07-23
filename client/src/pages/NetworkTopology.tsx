import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { getDevices, getLatestMetrics } from '../api/client';
import { getTopologyTree, type TopologyNode } from '../api/phase2';
import { useSocket } from '../hooks/useSocket';
import type { Device, Metric } from '../api/types';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import DeviceModal from '../components/DeviceModal';

const COLOR: Record<string, string> = {
  up: '#9bc86e', ok: '#9bc86e',
  warning: '#ebc434', degraded: '#ebc434',
  down: '#e86060', unknown: '#8c8c8c',
};
const STATUS_KEYS = Object.keys(COLOR) as (keyof typeof COLOR)[];
const colorFor = (s: string) => (STATUS_KEYS.includes(s as keyof typeof COLOR) ? COLOR[s as keyof typeof COLOR] : COLOR.unknown);

const CATEGORY_SHAPES: Record<string, (ctx: CanvasRenderingContext2D, x: number, y: number, r: number) => void> = {
  router: (ctx, x, y, r) => {
    // Circle with crosshair
    ctx.beginPath(); ctx.arc(x, y, r, 0, 2 * Math.PI); ctx.fill(); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(x - r * 0.6, y); ctx.lineTo(x + r * 0.6, y); ctx.moveTo(x, y - r * 0.6); ctx.lineTo(x, y + r * 0.6); ctx.stroke();
  },
  switch: (ctx, x, y, r) => {
    // Diamond
    ctx.beginPath(); ctx.moveTo(x, y - r); ctx.lineTo(x + r, y); ctx.lineTo(x, y + r); ctx.lineTo(x - r, y); ctx.closePath(); ctx.fill(); ctx.stroke();
  },
  firewall: (ctx, x, y, r) => {
    // Hexagon
    const a = r * 0.9;
    ctx.beginPath();
    for (let i = 0; i < 6; i++) { const angle = (Math.PI / 3) * i - Math.PI / 6; ctx.lineTo(x + a * Math.cos(angle), y + a * Math.sin(angle)); }
    ctx.closePath(); ctx.fill(); ctx.stroke();
  },
  server: (ctx, x, y, r) => {
    // Rounded rectangle
    const w = r * 1.6, h = r * 1.2;
    ctx.beginPath(); ctx.roundRect(x - w / 2, y - h / 2, w, h, 3); ctx.fill(); ctx.stroke();
  },
  default: (ctx, x, y, r) => {
    ctx.beginPath(); ctx.arc(x, y, r, 0, 2 * Math.PI); ctx.fill(); ctx.stroke();
  },
};

function getCategoryShape(category: string | undefined) {
  if (!category) return CATEGORY_SHAPES.default;
  const key = category.toLowerCase();
  if (key.includes('router') || key.includes('gateway')) return CATEGORY_SHAPES.router;
  if (key.includes('switch')) return CATEGORY_SHAPES.switch;
  if (key.includes('firewall')) return CATEGORY_SHAPES.firewall;
  if (key.includes('server') || key.includes('rack')) return CATEGORY_SHAPES.server;
  return CATEGORY_SHAPES.default;
}

interface SimNode { id: number; name: string; status: string; ip: string; category: string; x: number; y: number; vx: number; vy: number; fx?: number | null; fy?: number | null; r: number; }
interface SimLink { source: SimNode; target: SimNode; port?: string; }

function statusOf(device: Device, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(device.id)?.status || device.status || 'unknown';
}

/** Flatten the topology tree into a flat list of edges (parentId -> childId) */
function flattenTree(nodes: TopologyNode[]): Array<{ parentId: number; childId: number; port?: string }> {
  const edges: Array<{ parentId: number; childId: number; port?: string }> = [];
  function walk(node: TopologyNode) {
    if (node.children) {
      for (const child of node.children) {
        edges.push({ parentId: node.deviceId, childId: child.deviceId, port: child.dependencyPort });
        walk(child);
      }
    }
  }
  for (const root of nodes) walk(root);
  return edges;
}

export default function NetworkTopology() {
  const fgRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [metricsMap, setMetricsMap] = useState<Map<number, Metric>>(new Map());
  const [topoEdges, setTopoEdges] = useState<Array<{ parentId: number; childId: number; port?: string }>>([]);
  const [selected, setSelected] = useState<Device | null>(null);
  const [hovered, setHovered] = useState<{ node: SimNode } | null>(null);
  const [filters, setFilters] = useState({ status: 'all', protocol: 'all', category: 'all' });
  const [zoom, setZoom] = useState(1);
  const [size, setSize] = useState({ w: 900, h: 560 });
  const metricsRef = useRef(metricsMap);
  useEffect(() => { metricsRef.current = metricsMap; }, [metricsMap]);

  // Fetch data on mount
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [d, m, t] = await Promise.all([
          getDevices(),
          getLatestMetrics(),
          getTopologyTree().catch(() => ({ data: [] as TopologyNode[] })),
        ]);
        if (cancelled) return;
        setDevices(d.data || []);
        setMetricsMap(new Map((m.data || []).map((item: Metric) => [item.deviceId, item])));
        const treeData = t.data || [];
        setTopoEdges(flattenTree(treeData));
      } catch {
        if (!cancelled) setDevices([]);
      }
    })();
    return () => { cancelled = true; };
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
  const categories = useMemo(() => Array.from(new Set(devices.filter(d => d.deviceCategory).map((d) => d.deviceCategory!))).sort(), [devices]);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const cr = entries[0]?.contentRect;
      if (cr) setSize({ w: Math.max(280, Math.floor(cr.width)), h: 560 });
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
      if (filters.category !== 'all' && d.deviceCategory !== filters.category) return false;
      return true;
    });
    const ids = new Set(list.map((d) => d.id));
    return { list, ids };
  }, [devices, metricsMap, filters]);

  // Simulation
  const sim = useRef<{ nodes: SimNode[]; links: SimLink[]; raf: number; dragging: SimNode | null; mx: number; my: number; pan: { x: number; y: number } }>({ nodes: [], links: [], raf: 0, dragging: null, mx: 0, my: 0, pan: { x: 0, y: 0 } });

  useEffect(() => {
    const W = size.w, H = size.h;
    const prev = sim.current.nodes;
    const prevMap = new Map(prev.map((n) => [n.id, n]));
    const nodes: SimNode[] = visible.list.map((d, i) => {
      const ex = prevMap.get(d.id);
      const angle = (Math.PI * 2 * i) / Math.max(visible.list.length, 1);
      const radius = Math.min(W, H) * 0.3;
      return ex ? { ...ex, name: d.name, status: statusOf(d, metricsMap), ip: d.ipAddress, category: d.deviceCategory || '' } : {
        id: d.id, name: d.name, status: statusOf(d, metricsMap), ip: d.ipAddress, category: d.deviceCategory || '',
        x: W / 2 + Math.cos(angle) * radius, y: H / 2 + Math.sin(angle) * radius, vx: 0, vy: 0, r: 7,
      };
    });
    const nodeMap = new Map(nodes.map((n) => [n.id, n]));
    const links: SimLink[] = [];

    // 1. Real edges from the topology tree API
    for (const edge of topoEdges) {
      const src = nodeMap.get(edge.parentId);
      const tgt = nodeMap.get(edge.childId);
      if (src && tgt) links.push({ source: src, target: tgt, port: edge.port });
    }

    // 2. Supplement with parentDeviceId edges for devices not covered by the tree
    const linkedIds = new Set<number>();
    for (const l of links) { linkedIds.add(l.source.id); linkedIds.add(l.target.id); }
    for (const d of visible.list) {
      if (d.parentDeviceId && !linkedIds.has(d.id) && nodeMap.has(d.parentDeviceId)) {
        links.push({ source: nodeMap.get(d.parentDeviceId)!, target: nodeMap.get(d.id)! });
        linkedIds.add(d.id);
      }
    }

    // 3. Fallback hub-and-spoke for remaining orphans
    if (visible.list.length > 1) {
      const isHub = (d: Device) =>
        /gateway|router|core|switch|firewall/i.test(d.deviceCategory || '') ||
        /gateway|router|core|firewall/i.test(d.name || '');
      const hub = visible.list.find(isHub) ?? visible.list[0];
      const hubNode = nodeMap.get(hub.id)!;
      for (const d of visible.list) {
        if (d.id !== hub.id && !linkedIds.has(d.id) && nodeMap.has(hub.id)) {
          links.push({ source: hubNode, target: nodeMap.get(d.id)! });
        }
      }
    }

    sim.current.nodes = nodes;
    sim.current.links = links;
  }, [visible, metricsMap, size, topoEdges]);

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
        if (dev) { n.status = statusOf(dev, metricsRef.current); n.category = dev.deviceCategory || ''; }
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
          const force = 1800 / (dist * dist);
          const fx = (dx / dist) * force, fy = (dy / dist) * force;
          a.vx += fx; a.vy += fy; b.vx -= fx; b.vy -= fy;
        }
        // centering
        a.vx += (W / 2 - a.x) * 0.001;
        a.vy += (H / 2 - a.y) * 0.001;
      }
      // link springs
      for (const l of state.links) {
        const dx = l.target.x - l.source.x, dy = l.target.y - l.source.y;
        const dist = Math.hypot(dx, dy) || 0.01;
        const target = 100;
        const k = (dist - target) * 0.008;
        const fx = (dx / dist) * k, fy = (dy / dist) * k;
        if (l.source.fx == null) { l.source.vx += fx; l.source.vy += fy; }
        if (l.target.fx == null) { l.target.vx -= fx; l.target.vy -= fy; }
      }
      // integrate
      for (const n of nodes) {
        if (n.fx != null) continue;
        n.vx *= 0.85; n.vy *= 0.85;
        n.x += n.vx * (dt / 16); n.y += n.vy * (dt / 16);
        n.x = Math.max(n.r + 8, Math.min(W - n.r - 8, n.x));
        n.y = Math.max(n.r + 8, Math.min(H - n.r - 8, n.y));
      }
      // draw
      ctx.clearRect(0, 0, W, H);
      ctx.save();
      ctx.translate(state.pan.x, state.pan.y);
      ctx.scale(zoom, zoom);

      // edges
      const t = now / 600;
      for (const l of state.links) {
        const srcColor = colorFor(l.source.status);
        const tgtColor = colorFor(l.target.status);
        // Line
        ctx.beginPath();
        ctx.moveTo(l.source.x, l.source.y);
        ctx.lineTo(l.target.x, l.target.y);
        const isReal = l.port !== undefined || topoEdges.length > 0;
        ctx.strokeStyle = isReal ? 'rgba(155,200,110,0.25)' : 'rgba(140,140,130,0.15)';
        ctx.lineWidth = isReal ? 1.5 : 0.8;
        ctx.stroke();

        // Animated particle along edge
        const p = (t + (l.source.id + l.target.id) * 0.13) % 1;
        const px = l.source.x + (l.target.x - l.source.x) * p;
        const py = l.source.y + (l.target.y - l.source.y) * p;
        ctx.beginPath();
        ctx.arc(px, py, 1.8, 0, 2 * Math.PI);
        ctx.fillStyle = srcColor === COLOR.down || tgtColor === COLOR.down ? 'rgba(232,96,96,0.7)' : 'rgba(155,200,110,0.7)';
        ctx.fill();

        // Port label
        if (l.port) {
          const mx = (l.source.x + l.target.x) / 2;
          const my = (l.source.y + l.target.y) / 2;
          ctx.font = '8px "JetBrains Mono", monospace';
          ctx.textAlign = 'center'; ctx.textBaseline = 'bottom';
          ctx.fillStyle = 'rgba(180,180,170,0.6)';
          ctx.fillText(l.port, mx, my - 3);
        }
      }

      // nodes
      for (const n of nodes) {
        const c = colorFor(n.status);
        const drawShape = getCategoryShape(n.category);

        // Glow ring
        ctx.save();
        ctx.globalAlpha = 0.25;
        ctx.strokeStyle = c;
        ctx.lineWidth = 1;
        ctx.beginPath(); ctx.arc(n.x, n.y, n.r + 4, 0, 2 * Math.PI); ctx.stroke();
        ctx.restore();

        // Shape fill + stroke
        ctx.fillStyle = 'rgba(38,38,29,0.95)';
        ctx.strokeStyle = c;
        ctx.lineWidth = 1.8;
        drawShape(ctx, n.x, n.y, n.r);

        // Label
        const label = n.name.length > 16 ? `${n.name.slice(0, 15)}…` : n.name;
        ctx.font = '9px "Plus Jakarta Sans", sans-serif';
        ctx.textAlign = 'center'; ctx.textBaseline = 'top';
        ctx.fillStyle = 'rgba(225,225,215,0.9)';
        ctx.fillText(label, n.x, n.y + n.r + 5);
      }
      ctx.restore();
      state.raf = requestAnimationFrame(tick);
    };
    state.raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(state.raf);
  }, [size, zoom, nodeById, topoEdges]);

  const toWorld = (clientX: number, clientY: number) => {
    const rect = fgRef.current!.getBoundingClientRect();
    const x = (clientX - rect.left - sim.current.pan.x) / zoom;
    const y = (clientY - rect.top - sim.current.pan.y) / zoom;
    return { x, y };
  };
  const pick = (x: number, y: number) => {
    for (const n of sim.current.nodes) {
      if (Math.hypot(n.x - x, n.y - y) <= n.r + 5) return n;
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
  const onWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.08 : 0.92;
    setZoom((z) => Math.max(0.3, Math.min(4, +(z * factor).toFixed(2))));
  };

  const zoomBy = (factor: number) => setZoom((z) => Math.max(0.3, Math.min(4, +(z * factor).toFixed(2))));

  const edgeCount = sim.current.links.length;
  const realEdgeCount = topoEdges.length;

  return (
    <div className="space-y-5">
      <SectionHeader
        title="Network Topology"
        subtitle="Live force-directed dependency map built from real device relationships."
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
        <select value={filters.category} onChange={(e) => setFilters((f) => ({ ...f, category: e.target.value }))} className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-3 py-2 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All categories</option>
          {categories.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
        <div className="ml-auto flex items-center gap-2">
          <button onClick={() => zoomBy(1.2)} className="w-8 h-8 grid place-items-center rounded-md border border-outline-variant/30 text-on-surface-variant hover:text-on-surface hover:border-primary/50"><span className="material-symbols-outlined text-base">add</span></button>
          <button onClick={() => zoomBy(0.83)} className="w-8 h-8 grid place-items-center rounded-md border border-outline-variant/30 text-on-surface-variant hover:text-on-surface hover:border-primary/50"><span className="material-symbols-outlined text-base">remove</span></button>
          <button onClick={() => { setZoom(1); sim.current.pan = { x: 0, y: 0 }; }} className="px-3 h-8 rounded-md border border-outline-variant/30 text-xs text-on-surface-variant hover:text-on-surface hover:border-primary/50">Reset</button>
        </div>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[10px] uppercase tracking-wide text-on-surface-variant">
        <span className="text-success">● Up</span>
        <span className="text-warning">● Warning</span>
        <span className="text-error">● Down</span>
        <span className="border-l border-outline-variant/30 pl-5">◯ Host</span>
        <span>◇ Switch</span>
        <span>⬡ Firewall</span>
        <span>▢ Server</span>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1fr_280px] gap-5">
        <Card variant="low" className="p-4 overflow-hidden min-h-[600px] relative">
          <div className="text-[10px] uppercase tracking-wide text-on-surface-variant mb-3 flex items-center gap-3">
            <span className="flex items-center gap-1.5"><span className="w-1.5 h-1.5 rounded-full bg-success status-dot-live" /> Live topology</span>
            <span className="font-data">{visible.list.length} nodes</span>
            <span className="font-data">{realEdgeCount} dependency edges</span>
            {edgeCount > realEdgeCount && <span className="font-data text-outline">+{edgeCount - realEdgeCount} inferred</span>}
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
                onWheel={onWheel}
              />
            ) : (
              <div className="h-[540px] grid place-items-center text-sm text-on-surface-variant">No devices available for topology mapping.</div>
            )}
            {hovered && (
              <div className="absolute top-2 left-2 z-10 bg-surface-container-high border border-outline-variant/30 rounded-lg p-3 text-xs max-w-[240px] pointer-events-none">
                <strong className="font-headline text-sm block truncate">{hovered.node.name}</strong>
                <span className="text-on-surface-variant font-data">{hovered.node.ip}</span>
                <div className="mt-1.5 flex items-center gap-2">
                  <span className="flex items-center gap-1.5">
                    <span className="w-2 h-2 rounded-full" style={{ background: colorFor(hovered.node.status) }} />
                    <span className="uppercase tracking-wide" style={{ color: colorFor(hovered.node.status) }}>{hovered.node.status}</span>
                  </span>
                  {hovered.node.category && <span className="text-outline">· {hovered.node.category}</span>}
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
                {selected.deviceCategory && (
                  <div>
                    <dt className="text-[10px] uppercase text-on-surface-variant">Category</dt>
                    <dd className="capitalize">{selected.deviceCategory}</dd>
                  </div>
                )}
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Parent dependency</dt>
                  <dd>{selected.parentDeviceId ? (nodeById.get(selected.parentDeviceId)?.name || `Device #${selected.parentDeviceId}`) : 'Root / auto-discovered'}</dd>
                </div>
                <div>
                  <dt className="text-[10px] uppercase text-on-surface-variant">Response</dt>
                  <dd className="font-data">{metricsMap.get(selected.id)?.responseTime != null ? `${metricsMap.get(selected.id)!.responseTime}ms` : '—'}</dd>
                </div>
              </dl>
              <button onClick={() => setSelected(null)} className="mt-6 text-xs text-primary font-semibold hover:underline">Clear selection</button>
            </>
          ) : (
            <div className="text-sm text-on-surface-variant pt-12">Select a node to inspect its live status and dependency chain.</div>
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
