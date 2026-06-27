import { useMemo, useState, useCallback } from 'react';
import type { Phase2Row } from '../api/phase2';

interface LocationTreeProps {
  locations: Phase2Row[];
  onSelect?: (location: Phase2Row) => void;
  selectedId?: number | null;
  showStatus?: boolean;
  showDeviceCount?: boolean;
}

interface TreeNode extends Phase2Row {
  _children: TreeNode[];
  _depth: number;
}

const typeIcons: Record<string, string> = {
  campus: 'apartment',
  building: 'domain',
  floor: 'layers',
  room: 'meeting_room',
  rack: 'dns',
  zone: 'grid_view',
  closet: 'door_sliding',
  site: 'location_city',
};

function buildNodes(flat: Phase2Row[]): TreeNode[] {
  const byId = new Map<number, TreeNode>();
  for (const loc of flat) {
    const id = Number(loc.id);
    byId.set(id, { ...loc, _children: [], _depth: 0 });
  }
  const roots: TreeNode[] = [];
  for (const [, node] of byId) {
    const pid = node.parent_id != null ? Number(node.parent_id) : null;
    const parent = pid != null ? byId.get(pid) : null;
    if (parent) {
      parent._children.push(node);
    } else {
      roots.push(node);
    }
  }
  function setDepth(nodes: TreeNode[], d: number) {
    for (const n of nodes) {
      n._depth = d;
      setDepth(n._children, d + 1);
    }
  }
  setDepth(roots, 0);
  return roots;
}

function matchesSearch(node: TreeNode, needle: string): boolean {
  const name = String(node.name ?? '').toLowerCase();
  const code = String(node.code ?? '').toLowerCase();
  if (name.includes(needle) || code.includes(needle)) return true;
  return node._children.some((c) => matchesSearch(c, needle));
}

function StatusDots({ status }: { status: Record<string, unknown> }) {
  const up = Number(status.up ?? 0);
  const down = Number(status.down ?? 0);
  const warning = Number(status.warning ?? 0);
  if (up + down + warning === 0) return null;
  return (
    <span className="flex items-center gap-1 ml-auto shrink-0">
      {down > 0 && (
        <span className="flex items-center gap-0.5 text-[10px] font-bold text-error">
          <span className="w-2 h-2 rounded-full bg-error inline-block" />
          {down}
        </span>
      )}
      {warning > 0 && (
        <span className="flex items-center gap-0.5 text-[10px] font-bold text-warning">
          <span className="w-2 h-2 rounded-full bg-warning inline-block" />
          {warning}
        </span>
      )}
      {up > 0 && (
        <span className="flex items-center gap-0.5 text-[10px] font-bold text-success">
          <span className="w-2 h-2 rounded-full bg-success inline-block" />
          {up}
        </span>
      )}
    </span>
  );
}

function TreeItem({
  node,
  selectedId,
  onSelect,
  showStatus,
  showDeviceCount,
  searchOpen,
}: {
  node: TreeNode;
  selectedId?: number | null;
  onSelect?: (loc: Phase2Row) => void;
  showStatus?: boolean;
  showDeviceCount?: boolean;
  searchOpen: Set<number>;
}) {
  const hasChildren = node._children.length > 0;
  const isSelected = selectedId != null && Number(node.id) === selectedId;
  const [expanded, setExpanded] = useState(
    () => node._depth < 1 || searchOpen.has(Number(node.id)),
  );
  const icon = typeIcons[String(node.type)] || 'location_on';

  const toggleExpand = (e: React.MouseEvent) => {
    e.stopPropagation();
    setExpanded((p) => !p);
  };

  const handleClick = () => onSelect?.(node);

  const status =
    showStatus && node.status && typeof node.status === 'object'
      ? (node.status as Record<string, unknown>)
      : null;
  const devCount = showDeviceCount ? Number(node.device_count ?? 0) : 0;

  return (
    <li role="treeitem" aria-expanded={hasChildren ? expanded : undefined} aria-selected={isSelected}>
      <div
        role="button"
        tabIndex={0}
        onClick={handleClick}
        onKeyDown={(e) => e.key === 'Enter' && handleClick()}
        className={[
          'flex items-center gap-2 px-3 py-2 rounded-md cursor-pointer transition-all duration-150 group',
          isSelected
            ? 'bg-primary/15 text-primary ring-1 ring-primary/30'
            : 'hover:bg-surface-container-high/80 text-on-surface',
        ].join(' ')}
        style={{ paddingLeft: `${node._depth * 20 + 8}px` }}
      >
        {/* Expand toggle */}
        {hasChildren ? (
          <button
            onClick={toggleExpand}
            className="w-5 h-5 flex items-center justify-center shrink-0 rounded hover:bg-primary/10 transition-colors"
          >
            <span
              className={[
                'material-symbols-outlined text-sm transition-transform duration-200',
                expanded ? 'rotate-90' : '',
              ].join(' ')}
            >
              chevron_right
            </span>
          </button>
        ) : (
          <span className="w-5 shrink-0" />
        )}

        {/* Icon */}
        <span
          className={[
            'material-symbols-outlined text-base shrink-0',
            isSelected ? 'text-primary' : 'text-on-surface-variant group-hover:text-primary',
          ].join(' ')}
        >
          {icon}
        </span>

        {/* Name and code */}
        <div className="min-w-0 flex-1">
          <span className="font-headline font-bold text-sm truncate block">
            {String(node.name)}
          </span>
          {node.code ? (
            <span className="text-[10px] text-on-surface-variant uppercase tracking-wide">
              {String(node.code)}
            </span>
          ) : null}
        </div>

        {/* Device count badge */}
        {showDeviceCount && devCount > 0 && (
          <span className="text-[10px] bg-surface-container-highest text-on-surface-variant px-1.5 py-0.5 rounded-full font-bold shrink-0">
            {devCount}
          </span>
        )}

        {/* Status dots */}
        {status && <StatusDots status={status} />}
      </div>

      {/* Children */}
      {hasChildren && (
        <div
          className="overflow-hidden transition-all duration-200"
          style={{
            maxHeight: expanded ? `${node._children.length * 500}px` : '0',
            opacity: expanded ? 1 : 0,
          }}
        >
          <ul role="group">
            {node._children.map((child) => (
              <TreeItem
                key={Number(child.id)}
                node={child}
                selectedId={selectedId}
                onSelect={onSelect}
                showStatus={showStatus}
                showDeviceCount={showDeviceCount}
                searchOpen={searchOpen}
              />
            ))}
          </ul>
        </div>
      )}
    </li>
  );
}

export default function LocationTree({
  locations,
  onSelect,
  selectedId,
  showStatus = false,
  showDeviceCount = false,
}: LocationTreeProps) {
  const [search, setSearch] = useState('');

  const tree = useMemo(() => buildNodes(locations), [locations]);

  const needle = search.trim().toLowerCase();

  const filtered = useMemo(() => {
    if (!needle) return tree;
    return tree.filter((n) => matchesSearch(n, needle));
  }, [tree, needle]);

  // Collect IDs of nodes whose subtrees contain a match (to auto-expand).
  const searchOpen = useMemo(() => {
    const ids = new Set<number>();
    if (!needle) return ids;
    function collect(node: TreeNode): boolean {
      const nameMatch = String(node.name ?? '').toLowerCase().includes(needle);
      const codeMatch = String(node.code ?? '').toLowerCase().includes(needle);
      let childMatch = false;
      for (const c of node._children) {
        if (collect(c)) childMatch = true;
      }
      if (nameMatch || codeMatch || childMatch) {
        ids.add(Number(node.id));
        return true;
      }
      return false;
    }
    for (const n of tree) collect(n);
    return ids;
  }, [tree, needle]);

  const handleClear = useCallback(() => setSearch(''), []);

  return (
    <div className="flex flex-col h-full">
      {/* Search bar */}
      <div className="relative mb-3">
        <span className="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-base text-outline">
          search
        </span>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search locations..."
          className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg pl-9 pr-8 py-2 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none transition-[box-shadow]"
        />
        {search && (
          <button
            onClick={handleClear}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-on-surface-variant hover:text-primary"
          >
            <span className="material-symbols-outlined text-base">close</span>
          </button>
        )}
      </div>

      {/* Tree */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-8 text-on-surface-variant">
          <span className="material-symbols-outlined text-3xl mb-2">account_tree</span>
          <p className="text-xs uppercase tracking-wide">
            {needle ? 'No matching locations' : 'No locations yet'}
          </p>
        </div>
      ) : (
        <ul role="tree" className="flex-1 overflow-y-auto space-y-0.5">
          {filtered.map((node) => (
            <TreeItem
              key={Number(node.id)}
              node={node}
              selectedId={selectedId}
              onSelect={onSelect}
              showStatus={showStatus}
              showDeviceCount={showDeviceCount}
              searchOpen={searchOpen}
            />
          ))}
        </ul>
      )}
    </div>
  );
}
