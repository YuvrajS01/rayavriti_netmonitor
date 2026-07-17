import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getDevices, getLatestMetrics } from '../../api/client';
import { useAsyncEffect } from '../../hooks/useAsyncEffect';
import { useSocket } from '../../hooks/useSocket';
import type { Device, Metric } from '../../api/types';

const COLOR: Record<string, string> = {
  up: '#9bc86e', ok: '#9bc86e',
  warning: '#ebc434', degraded: '#ebc434',
  down: '#e86060', unknown: '#8c8c8c',
};
const colorFor = (s: string) => (COLOR[s as keyof typeof COLOR] ?? COLOR.unknown);

interface MiniNode { id: number; name: string; status: string; x: number; y: number; vx: number; vy: number; r: number; }
interface MiniLink { source: MiniNode; target: MiniNode; }

export default function NetworkTopologyMini() {
  const navigate = useNavigate();
  const fgRef = useRef<HTMLCanvasElement>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  const devicesRef = useRef<Device[]>([]);
  const metricsRef = useRef<Map<number, Metric>>(new Map());
  const sim = useRef<{ nodes: MiniNode[]; links: MiniLink[]; raf: number }>({ nodes: [], links: [], raf: 0 });
  const [count, setCount] = useState(0);
  const [width, setWidth] = useState(420);

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

  const rebuild = () => {
    const top = devicesRef.current.slice(0, 20);
    const nodes: MiniNode[] = top.map((d, i) => {
      const angle = (Math.PI * 2 * i) / Math.max(top.length, 1);
      return { id: d.id, name: d.name, status: metricsRef.current.get(d.id)?.status || d.status, x: 200 + Math.cos(angle) * 120, y: 100 + Math.sin(angle) * 80, vx: 0, vy: 0, r: 4 };
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
      const hubNode = nodeMap.get(hub.id)!;
      for (const d of top) {
        if (d.id !== hub.id) links.push({ source: hubNode, target: nodeMap.get(d.id)! });
      }
    }
    sim.current.nodes = nodes;
    sim.current.links = links;
    setCount(nodes.length);
  };

  useAsyncEffect(async (signal) => {
    const [d, m] = await Promise.all([getDevices(), getLatestMetrics()]);
    if (signal.aborted) return;
    devicesRef.current = d.data || [];
    metricsRef.current = new Map((m.data || []).map((item: Metric) => [item.deviceId, item]));
    rebuild();
  }, []);

  useSocket({
    onMetricUpdate: (metric) => {
      const mm = metric as unknown as Metric;
      metricsRef.current.set(mm.deviceId, mm);
      rebuild();
    },
  });

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
    let last = performance.now();
    const tick = (now: number) => {
      const dt = Math.min(32, now - last); last = now;
      const nodes = state.nodes;
      for (let i = 0; i < nodes.length; i++) {
        const a = nodes[i];
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j];
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
        ctx.strokeStyle = 'rgba(150,150,140,0.3)'; ctx.lineWidth = 0.8; ctx.stroke();
        const p = (t + (l.source.id + l.target.id) * 0.13) % 1;
        ctx.beginPath(); ctx.arc(l.source.x + (l.target.x - l.source.x) * p, l.source.y + (l.target.y - l.source.y) * p, 1.5, 0, 2 * Math.PI);
        ctx.fillStyle = 'rgba(155,200,110,0.8)'; ctx.fill();
      }
      for (const n of nodes) {
        const c = colorFor(n.status);
        ctx.beginPath(); ctx.arc(n.x, n.y, n.r + 1.5, 0, 2 * Math.PI); ctx.strokeStyle = c; ctx.globalAlpha = 0.4; ctx.lineWidth = 0.8; ctx.stroke();
        ctx.globalAlpha = 1; ctx.beginPath(); ctx.arc(n.x, n.y, n.r, 0, 2 * Math.PI);
        ctx.fillStyle = 'rgba(38,38,29,1)'; ctx.fill(); ctx.strokeStyle = c; ctx.lineWidth = 1.2; ctx.stroke();
      }
      state.raf = requestAnimationFrame(tick);
    };
    state.raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(state.raf);
  }, [width]);

  return (
    <section className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20 flex flex-col">
      <div className="flex items-center justify-between mb-3">
        <div>
          <h3 className="text-sm font-headline font-semibold uppercase tracking-wide">Topology preview</h3>
          <p className="text-[10px] uppercase tracking-wide text-on-surface-variant">Top {count} nodes · click to expand</p>
        </div>
        <button onClick={() => navigate('/devices/topology')} className="text-xs text-primary font-semibold hover:underline">Open full ↗</button>
      </div>
      <div ref={wrapRef} className="relative flex-1 min-h-[200px]">
        {count ? (
          <canvas ref={fgRef} className="cursor-pointer rounded" onClick={() => navigate('/devices/topology')} style={{ width, height: 210 }} />
        ) : (
          <div className="h-[210px] grid place-items-center text-xs text-on-surface-variant">Building topology…</div>
        )}
      </div>
    </section>
  );
}
