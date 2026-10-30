import { useMemo } from 'react';
import type { ResourceRow } from '../api/resources';

interface Place extends ResourceRow { status?: Record<string, number>; device_count?: number; }
interface Props { locations: Place[]; selectedId: number | null | undefined; onSelect: (location: Place) => void; }

interface HierarchicalPlace extends Place {
  children: HierarchicalPlace[];
}

const getDotColor = (status: Record<string, number> = {}, total: number) => {
  if (total === 0) return 'bg-line-strong';
  if ((status.down || 0) > 0) return 'bg-error';
  if ((status.warning || 0) > 0 || (status.degraded || 0) > 0) return 'bg-warning';
  if ((status.up || 0) > 0) return 'bg-success';
  return 'bg-line-strong';
};

const renderStatusBar = (status: Record<string, number> = {}, total: number) => {
  if (total === 0) return (
    <div className="flex w-full h-1.5 rounded-full overflow-hidden bg-surface-3 mt-3" />
  );
  const up = status.up || 0;
  const down = status.down || 0;
  const warning = (status.warning || 0) + (status.degraded || 0);
  
  return (
    <div className="flex w-full h-1.5 rounded-full overflow-hidden bg-surface-3 mt-3">
      {up > 0 && <div style={{ width: `${(up/total)*100}%` }} className="bg-success" />}
      {warning > 0 && <div style={{ width: `${(warning/total)*100}%` }} className="bg-warning" />}
      {down > 0 && <div style={{ width: `${(down/total)*100}%` }} className="bg-error" />}
    </div>
  );
};

function LocationNodeComponent({ node, selectedId, onSelect }: { node: HierarchicalPlace, selectedId: number | null | undefined, onSelect: (node: Place) => void }) {
  const type = String(node.type).toLowerCase();
  const isSelected = Number(node.id) === selectedId;
  const total = node.device_count || 0;
  const dotColor = getDotColor(node.status, total);

  if (type === 'campus') {
    return (
      <div className="flex flex-col gap-3 p-4 bg-surface-1 rounded-lg border border-line">
         <div className="flex items-center gap-2">
            <span className="material-symbols-outlined text-primary text-xl">apartment</span>
            <h3 className="font-sans font-bold text-lg text-fg">{String(node.name)}</h3>
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
      <button 
        onClick={(e) => { e.stopPropagation(); onSelect(node); }}
        className={`text-left w-full bg-surface-1 p-5 rounded-lg border transition-all hover:border-primary/50 ${isSelected ? 'border-primary ring-1 ring-primary/50 shadow-lg' : 'border-line hover:-translate-y-0.5 hover:shadow-md'}`}
      >
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-2">
             <div className={`w-3 h-3 rounded-full ${dotColor} shadow-sm`} />
             <span className="material-symbols-outlined text-primary text-xl">domain</span>
             <h3 className="font-sans font-bold text-lg text-fg">{String(node.name)}</h3>
          </div>
          <span className="text-[10px] uppercase tracking-wide text-fg-muted bg-surface-3 px-2 py-1 rounded">
            {total} nodes
          </span>
        </div>
        
        {renderStatusBar(node.status, total)}
        
        {node.children.length > 0 && (
          <div className="mt-5 flex flex-col gap-3">
            {node.children.map(child => (
              <LocationNodeComponent key={Number(child.id)} node={child} selectedId={selectedId} onSelect={onSelect} />
            ))}
          </div>
        )}
      </button>
    );
  }

  if (type === 'floor') {
    return (
      <div
        onClick={(e) => { e.stopPropagation(); onSelect(node); }}
        className={`bg-inset p-3.5 rounded-md border transition-all hover:border-primary/40 cursor-pointer flex flex-col gap-3 ${isSelected ? 'border-primary bg-surface-1' : 'border-line'}`}
      >
         <div className="flex items-center justify-between">
           <div className="flex items-center gap-2">
             <div className={`w-2 h-2 rounded-full ${dotColor}`} />
             <span className="material-symbols-outlined text-info text-base">layers</span>
             <span className="font-sans font-semibold text-sm text-fg">{String(node.name)}</span>
           </div>
           <span className="text-[10px] uppercase tracking-wide text-fg-muted">{total} nodes</span>
         </div>
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
    <div
      onClick={(e) => { e.stopPropagation(); onSelect(node); }}
      className={`flex items-center gap-1.5 px-3 py-1.5 rounded border hover:border-primary/40 cursor-pointer transition-colors ${isSelected ? 'border-primary bg-surface-2' : 'bg-surface-2 border-line'}`}
    >
      <div className={`w-1.5 h-1.5 rounded-full ${dotColor}`} />
      <span className="material-symbols-outlined text-[14px] text-fg-muted">{icon}</span>
      <span className="font-sans text-xs font-medium text-fg">{String(node.name)}</span>
      <span className="text-[10px] text-fg-muted ml-1">({total})</span>
    </div>
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
      <div className="relative min-h-[500px] overflow-hidden rounded-lg bg-inset border border-line p-7 flex items-center justify-center flex-col gap-4 text-fg-muted" aria-label="Campus schematic map">
        <div className="absolute inset-0 opacity-30 pointer-events-none" style={{ backgroundImage: 'linear-gradient(var(--color-line) 1px, transparent 1px), linear-gradient(90deg, var(--color-line) 1px, transparent 1px)', backgroundSize: '32px 32px' }} />
        <span className="material-symbols-outlined text-4xl relative z-10">map</span>
        <div className="text-sm relative z-10">Add locations to build a visual campus map.</div>
      </div>
    );
  }

  const campuses = roots.filter(r => String(r.type).toLowerCase() === 'campus');
  const otherRoots = roots.filter(r => String(r.type).toLowerCase() !== 'campus');

  return (
    <div className="relative min-h-[500px] overflow-hidden rounded-lg bg-inset border border-line p-7" aria-label="Campus schematic map">
      <div className="absolute inset-0 opacity-30 pointer-events-none" style={{ backgroundImage: 'linear-gradient(var(--color-line) 1px, transparent 1px), linear-gradient(90deg, var(--color-line) 1px, transparent 1px)', backgroundSize: '32px 32px' }} />
      
      <div className="relative z-10 flex flex-col gap-10 h-full overflow-y-auto pr-2 pb-4">
        {campuses.map(campus => (
          <div key={Number(campus.id)} className="flex flex-col gap-5">
            <div className="flex items-center gap-3">
               <span className="material-symbols-outlined text-primary text-3xl">apartment</span>
               <h2 className="font-sans text-2xl font-bold text-fg">{String(campus.name)}</h2>
               <span className="text-[10px] uppercase tracking-wide text-fg-muted bg-surface-2 px-2.5 py-1 rounded-md ml-1">
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
              <h2 className="font-sans text-xl font-bold text-fg">Other Locations</h2>
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
