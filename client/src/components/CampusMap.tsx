import type { Phase2Row } from '../api/phase2';

interface Place extends Phase2Row { status?: Record<string, number>; device_count?: number; }
interface Props { locations: Place[]; selectedId?: number | null; onSelect: (location: Place) => void; }
export default function CampusMap({ locations, selectedId, onSelect }: Props) {
  const visible = locations.filter(location => ['campus', 'building', 'floor', 'room'].includes(String(location.type))).slice(0, 18);
  return <div className="relative min-h-[500px] overflow-hidden rounded-lg bg-surface-container-lowest border border-outline-variant/20 p-7" aria-label="Campus schematic map">
    <div className="absolute inset-0 opacity-30" style={{ backgroundImage: 'linear-gradient(var(--color-outline-variant) 1px, transparent 1px), linear-gradient(90deg, var(--color-outline-variant) 1px, transparent 1px)', backgroundSize: '32px 32px' }} />
    <div className="relative grid grid-cols-2 md:grid-cols-3 gap-5 h-full">{visible.map((location, index) => { const status = location.status || {}; const failing = (status.down || 0) > 0; const warning = (status.warning || 0) > 0; const color = failing ? 'var(--color-error)' : warning ? 'var(--color-warning)' : 'var(--color-success)'; const selected = Number(location.id) === selectedId; return <button key={String(location.id)} onClick={() => onSelect(location)} className={`relative min-h-28 text-left bg-surface-container-low border p-4 transition-all hover:-translate-y-1 ${selected ? 'border-primary ring-1 ring-primary/50' : 'border-outline-variant/30 hover:border-outline/60'}`} style={{ transform: `translateY(${(index % 3) * 8}px)` }}><span className="absolute -top-2 -right-2 w-4 h-4 rounded-full border-4 border-surface-container-low" style={{ background: color }} /><span className="material-symbols-outlined text-primary">{location.type === 'building' ? 'domain' : location.type === 'floor' ? 'layers' : 'meeting_room'}</span><strong className="block font-headline text-sm mt-2 truncate">{String(location.name)}</strong><span className="text-[10px] text-on-surface-variant uppercase tracking-wide">{location.device_count || 0} monitored nodes</span></button>; })}</div>
    {!visible.length && <div className="relative h-[420px] grid place-items-center text-sm text-on-surface-variant">Add locations to build a visual campus map.</div>}
  </div>;
}
