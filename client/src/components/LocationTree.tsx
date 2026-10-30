import { useMemo, useState, useCallback } from 'react';
import type { ResourceRow } from '../api/resources';
import { asText } from '../utils/formatters';
import { STATUS_META, type Status } from '../utils/status';
import { StatusShapeIcon } from './ui/Status';
import IconButton from './ui/IconButton';

interface LocationTreeProps {
  locations: ResourceRow[];
  onSelect?: (location: ResourceRow) => void;
  selectedId?: number | null;
  showStatus?: boolean;
  showDeviceCount?: boolean;
}

interface TreeNode extends ResourceRow {
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

function buildNodes(flat: ResourceRow[]): TreeNode[] {
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
  const name = asText(node.name).toLowerCase();
  const code = asText(node.code).toLowerCase();
  if (name.includes(needle) || code.includes(needle)) return true;
  return node._children.some((c) => matchesSearch(c, needle));
}

function StatusDots({ status }: { status: Record<string, unknown> }) {
  const up = Number(status.up ?? 0);
  const down = Number(status.down ?? 0);
  const warning = Number(status.warning ?? 0);
  if (up + down + warning === 0) return null;
  const parts: [Status, number][] = [['down', down], ['warning', warning], ['up', up]];
  return (
    <span className="flex items-center gap-1.5 ml-auto shrink-0">
      {parts.filter(([, n]) => n > 0).map(([s, n]) => (
        <span key={s} className={`flex items-center gap-1 text-xs font-semibold tabular-nums ${STATUS_META[s].text}`}>
          <StatusShapeIcon shape={STATUS_META[s].shape} size={8} />
          {n}
          <span className="sr-only"> {STATUS_META[s].label.toLowerCase()}</span>
        </span>
      ))}
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
  onSelect?: (loc: ResourceRow) => void;
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
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleClick(); }
        }}
        className={[
          'flex items-center gap-2 px-3 py-2 rounded-md cursor-pointer transition-colors duration-[var(--duration-fast)] group',
          isSelected
            ? 'bg-surface-3 text-fg ring-1 ring-line-strong'
            : 'hover:bg-surface-2 text-fg',
        ].join(' ')}
        style={{ paddingLeft: `${node._depth * 20 + 8}px` }}
      >
        {/* Expand toggle */}
        {hasChildren ? (
          <button
            type="button"
            onClick={toggleExpand}
            aria-label={`${expanded ? 'Collapse' : 'Expand'} ${String(node.name)}`}
            aria-expanded={expanded}
            className="w-5 h-5 flex items-center justify-center shrink-0 rounded hover:bg-surface-3 transition-colors"
          >
            <span
              aria-hidden="true"
              className={[
                'material-symbols-outlined text-sm transition-transform duration-[var(--duration-normal)]',
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
          aria-hidden="true"
          className={[
            'material-symbols-outlined text-base shrink-0',
            isSelected ? 'text-fg' : 'text-fg-muted group-hover:text-fg',
          ].join(' ')}
        >
          {icon}
        </span>

        {/* Name and code */}
        <div className="min-w-0 flex-1">
          <span className="font-semibold text-sm truncate block">
            {String(node.name)}
          </span>
          {node.code ? (
            <span className="font-mono text-xs text-fg-muted">
              {asText(node.code)}
            </span>
          ) : null}
        </div>

        {/* Device count badge */}
        {showDeviceCount && devCount > 0 && (
          <span className="text-xs bg-surface-3 text-fg-muted px-1.5 py-0.5 rounded-full font-semibold tabular-nums shrink-0" aria-label={`${devCount} devices`}>
            {devCount}
          </span>
        )}

        {/* Status dots */}
        {status && <StatusDots status={status} />}
      </div>

      {/* Children */}
      {hasChildren && (
        <div
          className="overflow-hidden transition-[max-height,opacity] duration-[var(--duration-normal)]"
          inert={!expanded || undefined}
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
      const nameMatch = asText(node.name).toLowerCase().includes(needle);
      const codeMatch = asText(node.code).toLowerCase().includes(needle);
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
        <span className="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-base text-fg-subtle" aria-hidden="true">
          search
        </span>
        <input
          type="search"
          aria-label="Search locations"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search locations..."
          className="w-full bg-inset border border-line rounded-md pl-9 pr-9 py-2 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none transition-colors"
        />
        {search && (
          <IconButton
            icon="close"
            label="Clear search"
            size="sm"
            onClick={handleClear}
            className="absolute right-1 top-1/2 -translate-y-1/2"
          />
        )}
      </div>

      {/* Tree */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-8 text-fg-muted">
          <span className="material-symbols-outlined text-3xl mb-2 text-fg-subtle" aria-hidden="true">account_tree</span>
          <p className="text-sm">
            {needle ? 'No matching locations' : 'No locations yet'}
          </p>
        </div>
      ) : (
        <ul role="tree" aria-label="Locations" className="flex-1 overflow-y-auto space-y-0.5">
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
