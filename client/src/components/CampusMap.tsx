import { useMemo } from 'react';
import type { ResourceRow } from '../api/resources';
import { StatusDot } from './ui/Status';
import { STATUS_META, type Status } from '../utils/status';

interface Place extends ResourceRow { status?: Record<string, number>; device_count?: number; }
interface Props { locations: Place[]; selectedId: number | null | undefined; onSelect: (location: Place) => void; }

interface HierarchicalPlace extends Place {
  children: HierarchicalPlace[];
}

/** Worst status present at a location (no devices = unknown). */
const aggregateStatus = (status: Record<string, number> = {}, total: number): Status => {
  if (total === 0) return 'unknown';
  if ((status.down || 0) > 0) return 'down';
  if ((status.warning || 0) > 0 || (status.degraded || 0) > 0) return 'warning';
  if ((status.up || 0) > 0) return 'up';
  return 'unknown';
};

const renderStatusBar = (status: Record<string, number> = {}, total: number) => {
  if (total === 0) return (
    <div className="flex w-full h-1.5 rounded-full overflow-hidden bg-surface-3 mt-3" />
  );
  const up = status.up || 0;
  const down = status.down || 0;
  const warning = (status.warning || 0) + (status.degraded || 0);

  return (
    <div className="mt-3">
      <div className="flex w-full h-1.5 gap-0.5 rounded-full overflow-hidden bg-surface-3" aria-hidden="true">
        {up > 0 && <div style={{ width: `${(up/total)*100}%` }} className={STATUS_META.up.bg} />}
        {warning > 0 && <div style={{ width: `${(warning/total)*100}%` }} className={STATUS_META.warning.bg} />}
        {down > 0 && <div style={{ width: `${(down/total)*100}%` }} className={STATUS_META.down.bg} />}
      </div>
      <p className="mt-1.5 text-xs text-fg-muted">
        <span className="tabular-nums">{up}</span> up · <span className="tabular-nums">{warning}</span> warning · <span className="tabular-nums">{down}</span> down
      </p>
    </div>
  );
};

function LocationNodeComponent({ node, selectedId, onSelect }: { node: HierarchicalPlace, selectedId: number | null | undefined, onSelect: (node: Place) => void }) {
  const type = String(node.type).toLowerCase();
  const isSelected = Number(node.id) === selectedId;
  const total = node.device_count || 0;
  const status = aggregateStatus(node.status, total);

  if (type === 'campus') {
    return (
      <div className="flex flex-col gap-3 p-4 bg-surface-1 rounded-lg border border-line">
         <div className="flex items-center gap-2">
            <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">apartment</span>
            <h3 className="font-semibold text-lg text-fg">{String(node.name)}</h3>
         </div>
         <div className="grid grid-cols-1 gap-4">
           {node.children.map(child => (
             <LocationNodeComponent key={Number(child.id)} node={child} selectedId={selectedId} onSelect={onSelect} />
           ))}
         </div>
      </div>
    );
  }

  if (type === 'building') {
    return (
      <div
        className={`w-full bg-surface-1 p-5 rounded-lg border transition-colors ${isSelected ? 'border-primary ring-1 ring-primary' : 'border-line hover:border-line-strong'}`}
      >
        <button
          type="button"
          aria-pressed={isSelected}
          onClick={(e) => { e.stopPropagation(); onSelect(node); }}
          className="flex w-full items-start justify-between gap-3 mb-3 text-left rounded-md"
        >
          <div className="flex items-center gap-2 min-w-0">
             <StatusDot status={status} />
             <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">domain</span>
             <h3 className="font-semibold text-lg text-fg truncate">{String(node.name)}</h3>
          </div>
          <span className="shrink-0 text-xs text-fg-muted bg-surface-2 px-2 py-1 rounded tabular-nums">
            {total} nodes
          </span>
        </button>
        
        {renderStatusBar(node.status, total)}

        {node.children.length > 0 && (
          <div className="mt-5 flex flex-col gap-3">
            {node.children.map(child => (
              <LocationNodeComponent key={Number(child.id)} node={child} selectedId={selectedId} onSelect={onSelect} />
            ))}
          </div>
        )}
      </div>
    );
  }

  if (type === 'floor') {
    return (
      <div
        className={`bg-inset p-3.5 rounded-md border transition-colors flex flex-col gap-3 ${isSelected ? 'border-primary bg-surface-2' : 'border-line hover:border-line-strong'}`}
      >
         <button
           type="button"
           aria-pressed={isSelected}
           onClick={(e) => { e.stopPropagation(); onSelect(node); }}
           className="flex w-full items-center justify-between gap-2 text-left rounded"
         >
           <div className="flex items-center gap-2 min-w-0">
             <StatusDot status={status} size={8} />
             <span className="material-symbols-outlined text-fg-muted text-base" aria-hidden="true">layers</span>
             <span className="font-semibold text-sm text-fg truncate">{String(node.name)}</span>
           </div>
           <span className="shrink-0 text-xs text-fg-muted tabular-nums">{total} nodes</span>
         </button>
         {node.children.length > 0 && (
           <div className="flex flex-wrap gap-2">
             {node.children.map(child => (
               <LocationNodeComponent key={Number(child.id)} node={child} selectedId={selectedId} onSelect={onSelect} />
             ))}
           </div>
         )}
      </div>
    );
  }

  // Room, rack, etc.
  const icon = type === 'rack' ? 'dns' : 'meeting_room';
  return (
    <button
      type="button"
      aria-pressed={isSelected}
      onClick={(e) => { e.stopPropagation(); onSelect(node); }}
      className={`flex items-center gap-1.5 px-3 py-1.5 rounded border transition-colors ${isSelected ? 'border-primary bg-surface-3' : 'bg-surface-2 border-line hover:border-line-strong'}`}
    >
      <StatusDot status={status} size={8} />
      <span className="material-symbols-outlined text-[14px] text-fg-muted" aria-hidden="true">{icon}</span>
      <span className="text-xs font-medium text-fg">{String(node.name)}</span>
      <span className="text-xs text-fg-muted ml-1 tabular-nums">({total})</span>
    </button>
  );
}

export default function CampusMap({ locations, selectedId, onSelect }: Props) {
  const roots = useMemo(() => {
    const map = new Map<number, HierarchicalPlace>();
    const rootsArray: HierarchicalPlace[] = [];

    locations.forEach(loc => {
      map.set(Number(loc.id), { ...loc, children: [] });
    });

    locations.forEach(loc => {
      const node = map.get(Number(loc.id))!;
      if (loc.parent_id != null) {
        const parent = map.get(Number(loc.parent_id));
        if (parent) {
          parent.children.push(node);
        } else {
          rootsArray.push(node);
        }
      } else {
        rootsArray.push(node);
      }
    });

    return rootsArray;
  }, [locations]);

  if (locations.length === 0) {
    return (
      <div role="region" className="relative min-h-[500px] overflow-hidden rounded-lg bg-inset border border-line p-7 flex items-center justify-center flex-col gap-4 text-fg-muted" aria-label="Campus schematic map">
        <div className="absolute inset-0 opacity-30 pointer-events-none" style={{ backgroundImage: 'linear-gradient(var(--color-line) 1px, transparent 1px), linear-gradient(90deg, var(--color-line) 1px, transparent 1px)', backgroundSize: '32px 32px' }} />
        <span className="material-symbols-outlined text-4xl relative" aria-hidden="true">map</span>
        <div className="text-sm relative">Add locations to build a visual campus map.</div>
      </div>
    );
  }

  const campuses = roots.filter(r => String(r.type).toLowerCase() === 'campus');
  const otherRoots = roots.filter(r => String(r.type).toLowerCase() !== 'campus');

  return (
    <div role="region" className="relative min-h-[500px] overflow-hidden rounded-lg bg-inset border border-line p-7" aria-label="Campus schematic map">
      <div className="absolute inset-0 opacity-30 pointer-events-none" style={{ backgroundImage: 'linear-gradient(var(--color-line) 1px, transparent 1px), linear-gradient(90deg, var(--color-line) 1px, transparent 1px)', backgroundSize: '32px 32px' }} />

      <div className="relative flex flex-col gap-10 h-full overflow-y-auto pr-2 pb-4">
        {campuses.map(campus => (
          <div key={Number(campus.id)} className="flex flex-col gap-5">
            <div className="flex items-center gap-3">
               <span className="material-symbols-outlined text-fg-muted text-3xl" aria-hidden="true">apartment</span>
               <h2 className="text-2xl font-semibold text-fg">{String(campus.name)}</h2>
               <span className="text-xs text-fg-muted bg-surface-2 px-2.5 py-1 rounded-md ml-1 tabular-nums">
                 {campus.device_count || 0} nodes
               </span>
            </div>
            <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
              {campus.children.map(child => (
                <LocationNodeComponent key={Number(child.id)} node={child} selectedId={selectedId} onSelect={onSelect} />
              ))}
            </div>
          </div>
        ))}

        {otherRoots.length > 0 && (
          <div className="flex flex-col gap-5">
            {campuses.length > 0 && (
              <h2 className="text-xl font-semibold text-fg">Other locations</h2>
            )}
            <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
              {otherRoots.map(root => (
                <LocationNodeComponent key={Number(root.id)} node={root} selectedId={selectedId} onSelect={onSelect} />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
