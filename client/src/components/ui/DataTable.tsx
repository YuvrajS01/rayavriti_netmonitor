import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { nextSort, type Column, type FlatItem } from './dataTableModel';
import type { DataTableModel, RowId } from './useDataTable';
import Button from './Button';
import IconButton from './IconButton';
import Popover from './Popover';
import Dialog from './Dialog';

/** Above this many rows only the visible ones are rendered. */
export const VIRTUALIZE_AT = 150;
const ROW_HEIGHT = 44;
const GROUP_HEIGHT = 36;

const DEFAULT_TRACK = 'minmax(120px,1fr)';
const SELECT_TRACK = '44px';

const inputClass = 'h-8 w-full rounded-md border border-line bg-inset px-2 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';

// ── Toolbar ───────────────────────────────────────────────────────────────

interface ToolbarProps<T> {
  table: DataTableModel<T>;
  searchLabel: string;
  /** Extra controls on the right (view toggles, primary action). */
  children?: ReactNode;
  showFilters?: boolean;
  onToggleFilters?: () => void;
}

/** Search, saved views, group-by, filters and the column picker. */
export function DataTableToolbar<T>({ table, searchLabel, children, showFilters, onToggleFilters }: ToolbarProps<T>) {
  const [saving, setSaving] = useState(false);
  const [viewName, setViewName] = useState('');
  const viewSelectId = useId();
  const groupSelectId = useId();
  const current = table.views.find((v) => v.id === table.viewId);
  const hideable = table.columns.filter((c) => c.hideable !== false);
  const hasFilters = table.columns.some((c) => c.filter);
  const activeFilterCount = Object.values(table.state.filters).filter(Boolean).length;

  return (
    <div className="mb-3 flex flex-wrap items-end gap-2">
      <label className="relative min-w-48 flex-1">
        <span className="sr-only">{searchLabel}</span>
        <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
        <input
          type="search"
          value={table.state.query}
          onChange={(e) => table.setState({ query: e.target.value })}
          placeholder={searchLabel}
          className="h-control w-full rounded-md border border-line bg-surface-1 pl-9 pr-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary"
        />
      </label>

      <div className="flex items-center gap-1">
        <label htmlFor={viewSelectId} className="sr-only">View</label>
        <select
          id={viewSelectId}
          value={table.viewId}
          onChange={(e) => table.setView(e.target.value)}
          className="h-control rounded-md border border-line bg-surface-1 px-3 text-sm text-fg outline-none focus:border-primary"
        >
          <optgroup label="Views">
            {table.views.filter((v) => v.builtIn).map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
          </optgroup>
          {table.views.some((v) => !v.builtIn) && (
            <optgroup label="Saved views">
              {table.views.filter((v) => !v.builtIn).map((v) => <option key={v.id} value={v.id}>{v.label}</option>)}
            </optgroup>
          )}
        </select>
        {table.canSaveViews && table.dirty && (
          <Button variant="ghost" size="sm" icon="bookmark_add" onClick={() => { setViewName(''); setSaving(true); }}>Save view</Button>
        )}
        {current && !current.builtIn && (
          <IconButton icon="bookmark_remove" label={`Delete saved view ${current.label}`} variant="danger" size="sm" onClick={() => table.deleteView(current.id)} />
        )}
      </div>

      {table.groups.length > 0 && (
        <div className="flex items-center gap-2">
          <label htmlFor={groupSelectId} className="text-xs text-fg-muted">Group by</label>
          <select
            id={groupSelectId}
            value={table.state.groupBy ?? ''}
            onChange={(e) => table.setState({ groupBy: e.target.value || null })}
            className="h-control rounded-md border border-line bg-surface-1 px-3 text-sm text-fg outline-none focus:border-primary"
          >
            <option value="">None</option>
            {table.groups.map((g) => <option key={g.id} value={g.id}>{g.label}</option>)}
          </select>
        </div>
      )}

      {hasFilters && onToggleFilters && (
        <Button variant={showFilters ? 'primary-outline' : 'secondary'} icon="filter_list" onClick={onToggleFilters} aria-pressed={showFilters}>
          Filters{activeFilterCount > 0 ? ` (${activeFilterCount})` : ''}
        </Button>
      )}

      {hideable.length > 0 && (
        <Popover
          label="Visible columns"
          trigger={(props) => (
            <button
              type="button"
              {...props}
              className="inline-flex min-h-control items-center gap-2 rounded-md border border-line bg-surface-2 px-4 text-sm font-medium text-fg hover:border-line-strong hover:bg-surface-3"
            >
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">view_column</span>
              Columns
            </button>
          )}
        >
          <p className="px-2 pb-1 text-xs font-medium text-fg-muted">Show columns</p>
          <ul>
            {hideable.map((c) => {
              const shown = !table.state.hidden.includes(c.id);
              return (
                <li key={c.id}>
                  <label className="flex h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-sm text-fg hover:bg-surface-2">
                    <input
                      type="checkbox"
                      checked={shown}
                      onChange={() =>
                        table.setState({ hidden: shown ? [...table.state.hidden, c.id] : table.state.hidden.filter((h) => h !== c.id) })
                      }
                      className="accent-[var(--color-primary)]"
                    />
                    {c.header}
                  </label>
                </li>
              );
            })}
          </ul>
        </Popover>
      )}

      {children}

      <Dialog
        open={saving}
        onClose={() => setSaving(false)}
        size="sm"
        title="Save view"
        description="Saves the current search, filters, sort, grouping and columns in this browser."
        footer={
          <>
            <Button variant="secondary" onClick={() => setSaving(false)}>Cancel</Button>
            <Button type="submit" form="save-view-form" disabled={!viewName.trim()}>Save</Button>
          </>
        }
      >
        <form
          id="save-view-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!viewName.trim()) return;
            table.saveView(viewName);
            setSaving(false);
          }}
        >
          <label htmlFor="save-view-name" className="mb-1.5 block text-xs text-fg-muted">Name</label>
          <input id="save-view-name" value={viewName} onChange={(e) => setViewName(e.target.value)} maxLength={60} className="h-control w-full rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary" />
        </form>
      </Dialog>
    </div>
  );
}

// ── Table ─────────────────────────────────────────────────────────────────

interface DataTableProps<T> {
  table: DataTableModel<T>;
  /** Accessible name of the grid. */
  label: string;
  /** Text naming a row for the selection checkbox ("Select core-sw"). */
  rowLabel: (row: T) => string;
  selectable?: boolean;
  /** Enter on the active row (or double-click) opens it. */
  onRowOpen?: (row: T) => void;
  /** Rendered in the bulk bar while rows are selected. */
  bulkActions?: (rows: T[], clear: () => void) => ReactNode;
  showFilters?: boolean;
  /** Controlled active (keyboard) row. */
  activeId?: RowId | null;
  onActiveIdChange?: (id: RowId | null) => void;
  rowClassName?: (row: T) => string;
  groupHeader?: (group: Extract<FlatItem<T>, { kind: 'group' }>) => ReactNode;
  empty?: ReactNode;
  /** Max height of the scroll area (the header sticks inside it). */
  maxHeight?: string;
}

function ariaSort(table: DataTableModel<unknown>, column: Column<unknown>): 'ascending' | 'descending' | 'none' | undefined {
  if (!column.value || column.sortable === false) return undefined;
  if (table.state.sort?.id !== column.id) return 'none';
  return table.state.sort.desc ? 'descending' : 'ascending';
}

/**
 * Sortable, filterable, selectable grid with a sticky header, keyboard row
 * navigation (↑/↓, Home/End, PageUp/PageDown, Space selects, Enter opens)
 * and virtualised rendering for large lists. State lives in useDataTable.
 */
export function DataTable<T>({
  table,
  label,
  rowLabel,
  selectable = false,
  onRowOpen,
  bulkActions,
  showFilters = false,
  activeId: controlledActive,
  onActiveIdChange,
  rowClassName,
  groupHeader,
  empty,
  maxHeight = 'calc(100dvh - 320px)',
}: DataTableProps<T>) {
  const gridId = useId().replace(/[^a-zA-Z0-9_-]/g, '');
  const scrollRef = useRef<HTMLDivElement>(null);
  const [internalActive, setInternalActive] = useState<RowId | null>(null);
  const activeId = controlledActive !== undefined ? controlledActive : internalActive;
  const setActive = (id: RowId | null) => {
    if (onActiveIdChange) onActiveIdChange(id);
    if (controlledActive === undefined) setInternalActive(id);
  };

  const { items, rows, visibleColumns, getRowId } = table;
  const template = useMemo(
    () => [...(selectable ? [SELECT_TRACK] : []), ...visibleColumns.map((c) => c.width ?? DEFAULT_TRACK)].join(' '),
    [selectable, visibleColumns],
  );
  const rowIndexById = useMemo(() => new Map(rows.map((r, i) => [getRowId(r), i])), [rows, getRowId]);
  const itemIndexById = useMemo(() => {
    const map = new Map<RowId, number>();
    items.forEach((item, i) => { if (item.kind === 'row') map.set(getRowId(item.row), i); });
    return map;
  }, [items, getRowId]);

  const virtual = items.length > VIRTUALIZE_AT;
  // The compiler skips this component (useVirtualizer returns unmemoisable
  // functions); nothing here relies on compiler memoisation.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: virtual ? items.length : 0,
    getScrollElement: () => scrollRef.current,
    estimateSize: (i) => (items[i]?.kind === 'group' ? GROUP_HEIGHT : ROW_HEIGHT),
    overscan: 12,
  });

  // Keep the active row in view (keyboard navigation, j/k triage).
  useEffect(() => {
    if (activeId == null) return;
    const index = itemIndexById.get(activeId);
    if (index == null) return;
    if (virtual) virtualizer.scrollToIndex(index, { align: 'auto' });
    else document.getElementById(`${gridId}-row-${activeId}`)?.scrollIntoView?.({ block: 'nearest' });
  }, [activeId, itemIndexById, virtual, virtualizer, gridId]);

  const moveActive = (delta: number | 'start' | 'end') => {
    if (rows.length === 0) return;
    const current = activeId != null ? rowIndexById.get(activeId) ?? -1 : -1;
    let next: number;
    if (delta === 'start') next = 0;
    else if (delta === 'end') next = rows.length - 1;
    else next = Math.min(rows.length - 1, Math.max(0, current + delta));
    // Grouping reorders rows: walk the flattened order instead.
    if (table.state.groupBy) {
      const ordered = items.filter((i): i is Extract<FlatItem<T>, { kind: 'row' }> => i.kind === 'row');
      const pos = activeId != null ? ordered.findIndex((i) => getRowId(i.row) === activeId) : -1;
      const target = delta === 'start' ? 0 : delta === 'end' ? ordered.length - 1 : Math.min(ordered.length - 1, Math.max(0, pos + delta));
      setActive(getRowId(ordered[target]!.row));
      return;
    }
    setActive(getRowId(rows[next]!));
  };

  const activeRow = activeId != null ? rows[rowIndexById.get(activeId) ?? -1] : undefined;

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return; // typing in a cell control
    const page = Math.max(1, Math.floor((scrollRef.current?.clientHeight ?? 440) / ROW_HEIGHT) - 1);
    switch (event.key) {
      case 'ArrowDown': moveActive(1); break;
      case 'ArrowUp': moveActive(-1); break;
      case 'PageDown': moveActive(page); break;
      case 'PageUp': moveActive(-page); break;
      case 'Home': moveActive('start'); break;
      case 'End': moveActive('end'); break;
      case ' ':
        if (!selectable || !activeRow) return;
        table.toggleRow(activeRow);
        break;
      case 'Enter':
        if (!onRowOpen || !activeRow) return;
        onRowOpen(activeRow);
        break;
      default:
        return;
    }
    event.preventDefault();
  };

  const allVisibleSelected = rows.length > 0 && table.selectedRows.length === rows.length;
  const someSelected = table.selectedRows.length > 0 && !allVisibleSelected;

  const renderRow = (item: FlatItem<T>, style?: React.CSSProperties) => {
    if (item.kind === 'group') {
      return (
        <div key={`g-${item.key}`} role="row" style={style} className="flex items-center border-b border-line bg-surface-2 px-3 text-sm font-semibold text-fg" data-group={item.key}>
          <div role="gridcell" aria-colspan={visibleColumns.length + (selectable ? 1 : 0)} className="flex min-w-0 flex-1 items-center gap-2" style={{ height: GROUP_HEIGHT }}>
            {groupHeader ? groupHeader(item) : <>{item.label}<span className="font-normal text-fg-muted tabular-nums">{item.rows.length}</span></>}
          </div>
        </div>
      );
    }
    const { row } = item;
    const id = getRowId(row);
    const selected = selectable && table.isSelected(row);
    const isActive = activeId === id;
    return (
      <div
        key={`r-${id}`}
        id={`${gridId}-row-${id}`}
        role="row"
        aria-rowindex={item.index + 2}
        aria-selected={selectable ? selected : undefined}
        data-active={isActive || undefined}
        onClick={() => setActive(id)}
        onDoubleClick={() => onRowOpen?.(row)}
        style={{ ...style, gridTemplateColumns: template }}
        className={`grid items-center border-b border-line text-sm transition-colors duration-[var(--duration-fast)] ${
          selected ? 'bg-surface-2' : 'hover:bg-surface-2'
        } ${isActive ? 'shadow-[inset_3px_0_0_var(--color-primary)] bg-surface-2' : ''} ${rowClassName?.(row) ?? ''}`}
      >
        {selectable && (
          <div role="gridcell" className="flex items-center justify-center" style={{ height: ROW_HEIGHT }}>
            <input
              type="checkbox"
              tabIndex={-1}
              checked={selected}
              onChange={() => table.toggleRow(row)}
              onClick={(e) => e.stopPropagation()}
              aria-label={`Select ${rowLabel(row)}`}
              className="h-4 w-4 accent-[var(--color-primary)]"
            />
          </div>
        )}
        {visibleColumns.map((c) => (
          <div key={c.id} role="gridcell" className={`min-w-0 truncate px-3 ${c.align === 'end' ? 'text-right tabular-nums' : ''}`}>
            {c.cell ? c.cell(row) : String(c.value?.(row) ?? '—')}
          </div>
        ))}
      </div>
    );
  };

  return (
    <div>
      {selectable && table.selectedRows.length > 0 && (
        <div role="region" aria-label="Bulk actions" className="mb-2 flex flex-wrap items-center gap-3 rounded-lg border border-line-strong bg-surface-2 px-4 py-2">
          <span className="text-sm font-medium text-fg tabular-nums" aria-live="polite">{table.selectedRows.length} selected</span>
          <div className="flex flex-wrap items-center gap-2">{bulkActions?.(table.selectedRows, table.clearSelection)}</div>
          <Button variant="ghost" size="sm" onClick={table.clearSelection} className="ml-auto">Clear selection</Button>
        </div>
      )}
      <div
        ref={scrollRef}
        role="grid"
        aria-label={label}
        aria-rowcount={rows.length + 1}
        aria-colcount={visibleColumns.length + (selectable ? 1 : 0)}
        aria-multiselectable={selectable || undefined}
        aria-activedescendant={activeId != null && itemIndexById.has(activeId) ? `${gridId}-row-${activeId}` : undefined}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onFocus={() => { if (activeId == null && rows[0]) setActive(getRowId(rows[0])); }}
        className="relative overflow-auto rounded-lg border border-line bg-surface-1"
        style={{ maxHeight }}
      >
        <div role="rowgroup" className="sticky top-0 z-sticky bg-surface-1">
          <div role="row" aria-rowindex={1} className="grid border-b border-line-strong" style={{ gridTemplateColumns: template }}>
            {selectable && (
              <div role="columnheader" className="flex h-10 items-center justify-center">
                <input
                  type="checkbox"
                  checked={allVisibleSelected}
                  ref={(el) => { if (el) el.indeterminate = someSelected; }}
                  onChange={() => table.setRowsSelected(rows, !allVisibleSelected)}
                  aria-label={allVisibleSelected ? 'Deselect all rows' : `Select all ${rows.length} rows`}
                  className="h-4 w-4 accent-[var(--color-primary)]"
                />
              </div>
            )}
            {visibleColumns.map((c) => {
              const sort = ariaSort(table as DataTableModel<unknown>, c as Column<unknown>);
              return (
                <div key={c.id} role="columnheader" aria-sort={sort} className={`flex h-10 min-w-0 items-center px-3 text-xs font-semibold text-fg-muted ${c.align === 'end' ? 'justify-end' : ''}`}>
                  {sort ? (
                    <button
                      type="button"
                      onClick={() => table.setState({ sort: nextSort(table.state.sort, c.id) })}
                      className="inline-flex min-w-0 items-center gap-1 rounded-sm hover:text-fg"
                    >
                      <span className="truncate">{c.header}</span>
                      <span className="material-symbols-outlined text-[16px]" aria-hidden="true">
                        {sort === 'ascending' ? 'arrow_upward' : sort === 'descending' ? 'arrow_downward' : 'unfold_more'}
                      </span>
                    </button>
                  ) : (
                    <span className="truncate">{c.header}</span>
                  )}
                </div>
              );
            })}
          </div>
          {showFilters && (
            <div role="row" className="grid border-b border-line bg-surface-1 py-1.5" style={{ gridTemplateColumns: template }}>
              {selectable && <div role="columnheader" aria-label="No filter" />}
              {visibleColumns.map((c) => (
                <div key={c.id} role="columnheader" aria-label={`Filter ${c.header}`} className="min-w-0 px-2">
                  {c.filter === 'text' ? (
                    <input
                      type="search"
                      aria-label={`Filter by ${c.header.toLowerCase()}`}
                      placeholder="Filter…"
                      value={table.state.filters[c.id] ?? ''}
                      onChange={(e) => table.setState({ filters: { ...table.state.filters, [c.id]: e.target.value } })}
                      className={inputClass}
                    />
                  ) : Array.isArray(c.filter) ? (
                    <select
                      aria-label={`Filter by ${c.header.toLowerCase()}`}
                      value={table.state.filters[c.id] ?? ''}
                      onChange={(e) => table.setState({ filters: { ...table.state.filters, [c.id]: e.target.value } })}
                      className={inputClass}
                    >
                      <option value="">Any</option>
                      {(c.filter as ReadonlyArray<{ value: string; label: string }>).map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                    </select>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </div>

        {rows.length === 0 ? (
          <div role="row"><div role="gridcell" className="px-4">{empty}</div></div>
        ) : virtual ? (
          <div role="rowgroup" style={{ height: virtualizer.getTotalSize(), position: 'relative' }}>
            {virtualizer.getVirtualItems().map((v) =>
              renderRow(items[v.index]!, { position: 'absolute', top: 0, left: 0, width: '100%', height: v.size, transform: `translateY(${v.start}px)` }),
            )}
          </div>
        ) : (
          <div role="rowgroup">{items.map((item) => renderRow(item, item.kind === 'row' ? { minHeight: ROW_HEIGHT } : undefined))}</div>
        )}
      </div>
      <p className="mt-2 text-xs text-fg-muted tabular-nums">
        {rows.length} {rows.length === 1 ? 'row' : 'rows'}
        {selectable && ' · Space selects · Enter opens'}
      </p>
    </div>
  );
}
