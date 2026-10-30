import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getDevices, getLatestMetrics } from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import type { Device, Metric } from '../../api/types';
import { STATUS_META, normalizeStatus, type Status, type StatusShape } from '../../utils/status';
import { readToken, readTokenAlpha } from '../../utils/tokens';
import { THEME_CHANGE_EVENT } from '../../theme/theme';
import { prefersReducedMotion } from '../../hooks/useReducedMotion';
import { StatusShapeIcon } from '../ui/Status';

/** Canvas can't use var(): resolve tokens at draw time, re-read on theme change. */
function readPalette() {
  return {
    status: Object.fromEntries((Object.keys(STATUS_META) as Status[]).map((st) => [st, readToken(STATUS_META[st].token)])) as Record<Status, string>,
    edge: readToken('line-strong'),
    particle: readTokenAlpha('fg-muted', 0.7),
  };
}

/** Each node is drawn as its status shape, so the preview reads without colour. */
function drawNode(ctx: CanvasRenderingContext2D, shape: StatusShape, x: number, y: number, r: number) {
  ctx.beginPath();
  if (shape === 'triangle') { ctx.moveTo(x, y - r * 1.2); ctx.lineTo(x + r * 1.15, y + r * 0.9); ctx.lineTo(x - r * 1.15, y + r * 0.9); ctx.closePath(); ctx.fill(); }
  else if (shape === 'square') { ctx.rect(x - r * 0.9, y - r * 0.9, r * 1.8, r * 1.8); ctx.fill(); }
  else if (shape === 'ring') { ctx.arc(x, y, r * 0.85, 0, 2 * Math.PI); ctx.lineWidth = 1.6; ctx.stroke(); }
  else if (shape === 'pause') { ctx.rect(x - r * 0.9, y - r, r * 0.65, r * 2); ctx.rect(x + r * 0.25, y - r, r * 0.65, r * 2); ctx.fill(); }
  else if (shape === 'dash') { ctx.rect(x - r, y - r * 0.35, r * 2, r * 0.7); ctx.fill(); }
  else { ctx.arc(x, y, r, 0, 2 * Math.PI); ctx.fill(); }
}

const LEGEND = ['up', 'warning', 'down', 'unknown'] as const;

interface MiniNode { id: number; name: string; status: string; x: number; y: number; vx: number; vy: number; r: number; }
interface MiniLink { source: MiniNode; target: MiniNode; }

const MAX_NODES = 20;

export default function NetworkTopologyMini() {
  const navigate = useNavigate();
  const fgRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const sim = useRef<{ nodes: MiniNode[]; links: MiniLink[]; raf: number }>({ nodes: [], links: [], raf: 0 });
  const [width, setWidth] = useState(420);

  // Shared caches, kept live by useLiveQueryBridge.
  const devicesQuery = useQuery({
    queryKey: queryKeys.devices.list(),
    queryFn: ({ signal }) => getDevices({ signal }),
  });
  const metricsQuery = useQuery({
    queryKey: queryKeys.metrics.latest,
    queryFn: ({ signal }) => getLatestMetrics({ signal }),
  });
  const devices = devicesQuery.data?.data;
  const latestMetrics = metricsQuery.data?.data;
  const count = Math.min(devices?.length ?? 0, MAX_NODES);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w) setWidth(Math.max(280, Math.floor(w)));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    const metricsById = new Map((latestMetrics || []).map((item: Metric) => [item.deviceId, item]));
    const top = (devices || []).slice(0, MAX_NODES);
    const nodes: MiniNode[] = top.map((d, i) => {
      const angle = (Math.PI * 2 * i) / Math.max(top.length, 1);
      return { id: d.id, name: d.name, status: metricsById.get(d.id)?.status || d.status, x: 200 + Math.cos(angle) * 120, y: 100 + Math.sin(angle) * 80, vx: 0, vy: 0, r: 4 };
    });
    const nodeMap = new Map(nodes.map((n) => [n.id, n]));
    const links: MiniLink[] = [];
    for (const d of top) {
      if (d.parentDeviceId && nodeMap.has(d.parentDeviceId)) links.push({ source: nodeMap.get(d.parentDeviceId)!, target: nodeMap.get(d.id)! });
    }
    if (links.length === 0 && top.length > 1) {
      const isHub = (d: Device) =>
        /gateway|router|core|switch|firewall/i.test(d.deviceCategory || '') ||
        /gateway|router|core|firewall/i.test(d.name || '');
      const hub = top.find(isHub) ?? top[0];
      const hubNode = hub && nodeMap.get(hub.id);
      if (hub && hubNode) {
        for (const d of top) {
          if (d.id !== hub.id) links.push({ source: hubNode, target: nodeMap.get(d.id)! });
        }
      }
    }
    sim.current.nodes = nodes;
    sim.current.links = links;
  }, [devices, latestMetrics]);

  useEffect(() => {
    const canvas = fgRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const W = width, H = 210;
    canvas.width = W * dpr; canvas.height = H * dpr;
    canvas.style.width = `${W}px`; canvas.style.height = `${H}px`;
    ctx.scale(dpr, dpr);
    const state = sim.current;
    let palette = readPalette();
    const refreshPalette = () => { palette = readPalette(); };
    window.addEventListener(THEME_CHANGE_EVENT, refreshPalette);
    const reducedMotion = prefersReducedMotion();
    let last = performance.now();
    const tick = (now: number) => {
      const dt = Math.min(32, now - last); last = now;
      const nodes = state.nodes;
      for (let i = 0; i < nodes.length; i++) {
        const a = nodes[i];
        if (!a) continue;
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j];
          if (!b) continue;
          const dx = a.x - b.x, dy = a.y - b.y;
          const dist = Math.hypot(dx, dy) || 0.01;
          const force = 700 / (dist * dist);
          a.vx += (dx / dist) * force; a.vy += (dy / dist) * force;
          b.vx -= (dx / dist) * force; b.vy -= (dy / dist) * force;
        }
        a.vx += (W / 2 - a.x) * 0.001; a.vy += (H / 2 - a.y) * 0.001;
      }
      for (const l of state.links) {
        const dx = l.target.x - l.source.x, dy = l.target.y - l.source.y;
        const dist = Math.hypot(dx, dy) || 0.01;
        const k = (dist - 60) * 0.01;
        l.source.vx += (dx / dist) * k; l.source.vy += (dy / dist) * k;
        l.target.vx -= (dx / dist) * k; l.target.vy -= (dy / dist) * k;
      }
      for (const n of nodes) {
        n.vx *= 0.85; n.vy *= 0.85;
        n.x += n.vx * (dt / 16); n.y += n.vy * (dt / 16);
        n.x = Math.max(n.r + 4, Math.min(W - n.r - 4, n.x));
        n.y = Math.max(n.r + 4, Math.min(H - n.r - 4, n.y));
      }
      ctx.clearRect(0, 0, W, H);
      const t = now / 600;
      for (const l of state.links) {
        ctx.beginPath(); ctx.moveTo(l.source.x, l.source.y); ctx.lineTo(l.target.x, l.target.y);
        ctx.strokeStyle = palette.edge; ctx.lineWidth = 1; ctx.stroke();
        if (!reducedMotion) {
          const p = (t + (l.source.id + l.target.id) * 0.13) % 1;
          ctx.beginPath(); ctx.arc(l.source.x + (l.target.x - l.source.x) * p, l.source.y + (l.target.y - l.source.y) * p, 1.5, 0, 2 * Math.PI);
          ctx.fillStyle = palette.particle; ctx.fill();
        }
      }
      for (const n of nodes) {
        const meta = STATUS_META[normalizeStatus(n.status)];
        const c = palette.status[meta.status];
        ctx.fillStyle = c; ctx.strokeStyle = c;
        drawNode(ctx, meta.shape, n.x, n.y, n.r + 1);
      }
      state.raf = requestAnimationFrame(tick);
    };
    state.raf = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(state.raf);
      window.removeEventListener(THEME_CHANGE_EVENT, refreshPalette);
    };
  }, [width]);

  return (
    <section aria-labelledby="topology-preview-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
      <div className="flex items-center justify-between gap-3 mb-3">
        <div>
          <h3 id="topology-preview-heading" className="text-sm font-semibold text-fg">Topology preview</h3>
          <p className="text-xs text-fg-muted">Top {count} nodes</p>
        </div>
        <button type="button" onClick={() => navigate('/devices/topology')} className="inline-flex items-center gap-1 rounded-sm text-xs font-semibold text-primary hover:underline">
          Open full map
          <span className="material-symbols-outlined text-[16px]" aria-hidden="true">arrow_forward</span>
        </button>
      </div>
      <div ref={wrapRef} className="relative flex-1 min-h-[200px]">
        {count ? (
          // Decorative preview; the button above is the accessible way in.
          <canvas ref={fgRef} aria-hidden="true" className="cursor-pointer rounded" onClick={() => navigate('/devices/topology')} style={{ width, height: 210 }} />
        ) : (
          <div className="h-[210px] grid place-items-center text-xs text-fg-muted">Building topology…</div>
        )}
      </div>
      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-fg-muted">
        {LEGEND.map((st) => (
          <span key={st} className="inline-flex items-center gap-1">
            <span className={STATUS_META[st].text}><StatusShapeIcon shape={STATUS_META[st].shape} size={8} /></span>
            {STATUS_META[st].label}
          </span>
        ))}
      </div>
    </section>
  );
}
