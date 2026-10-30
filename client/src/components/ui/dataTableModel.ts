/**
 * Pure model behind <DataTable>: column definitions, view state, filtering,
 * sorting, grouping and saved-view persistence. Kept free of React so it
 * can be unit-tested and reused (the Devices card view renders the same
 * filtered rows as the table).
 */
import type { ReactNode } from 'react';

export type CellValue = string | number | boolean | null | undefined;

export interface Column<T> {
  id: string;
  header: string;
  /** Value used for sorting, filtering, search and the default cell. */
  value?: (row: T) => CellValue;
  cell?: (row: T) => ReactNode;
  /** Defaults to true when `value` is set. */
  sortable?: boolean;
  /** Column filter: free text, or a fixed list of options. */
  filter?: 'text' | ReadonlyArray<{ value: string; label: string }>;
  /** CSS grid track, e.g. 'minmax(160px,2fr)' or '96px'. */
  width?: string;
  align?: 'start' | 'end';
  /** False keeps it out of the column picker (always shown). */
  hideable?: boolean;
  /** Include in the global search (default true when `value` is set). */
  searchable?: boolean;
}

export interface SortState {
  id: string;
  desc: boolean;
}

export interface TableState {
  sort: SortState | null;
  filters: Record<string, string>;
  query: string;
  hidden: string[];
  groupBy: string | null;
}

export const EMPTY_TABLE_STATE: TableState = { sort: null, filters: {}, query: '', hidden: [], groupBy: null };

export interface GroupOption<T> {
  id: string;
  label: string;
  key: (row: T) => string;
  /** Orders group keys (default: alphabetical). */
  compare?: (a: string, b: string) => number;
}

export interface TableView<T> {
  id: string;
  label: string;
  state: Partial<TableState>;
  /** Built-in views can narrow rows beyond column filters (e.g. "Cameras"). */
  predicate?: (row: T) => boolean;
  /** Built-in views can't be deleted. */
  builtIn?: boolean;
}

export function normaliseState(partial: Partial<TableState> | undefined): TableState {
  return {
    sort: partial?.sort ?? null,
    filters: { ...(partial?.filters ?? {}) },
    query: partial?.query ?? '',
    hidden: [...(partial?.hidden ?? [])],
    groupBy: partial?.groupBy ?? null,
  };
}

export function sameState(a: TableState, b: TableState): boolean {
  const filters = (f: Record<string, string>) => JSON.stringify(Object.entries(f).filter(([, v]) => v !== '').sort());
  return (
    a.query === b.query &&
    a.groupBy === b.groupBy &&
    a.sort?.id === b.sort?.id &&
    Boolean(a.sort?.desc) === Boolean(b.sort?.desc) &&
    filters(a.filters) === filters(b.filters) &&
    [...a.hidden].sort().join() === [...b.hidden].sort().join()
  );
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

export function compareValues(a: CellValue, b: CellValue): number {
  const aEmpty = a == null || a === '';
  const bEmpty = b == null || b === '';
  if (aEmpty || bEmpty) return aEmpty === bEmpty ? 0 : aEmpty ? 1 : -1; // empty last
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return collator.compare(String(a), String(b));
}

/** Filter (view predicate, column filters, global query) then sort. */
export function processRows<T>(data: readonly T[], columns: readonly Column<T>[], state: TableState, predicate?: (row: T) => boolean): T[] {
  const byId = new Map(columns.map((c) => [c.id, c]));
  const activeFilters = Object.entries(state.filters)
    .filter(([id, v]) => v !== '' && byId.get(id)?.value)
    .map(([id, v]) => ({ column: byId.get(id)!, needle: v.toLowerCase(), exact: Array.isArray(byId.get(id)!.filter) }));
  const searchable = columns.filter((c) => c.value && c.searchable !== false);
  const query = state.query.trim().toLowerCase();

  let rows = predicate ? data.filter(predicate) : [...data];
  if (activeFilters.length) {
    rows = rows.filter((row) =>
      activeFilters.every(({ column, needle, exact }) => {
        const v = String(column.value!(row) ?? '').toLowerCase();
        return exact ? v === needle : v.includes(needle);
      }),
    );
  }
  if (query) {
    rows = rows.filter((row) => searchable.some((c) => String(c.value!(row) ?? '').toLowerCase().includes(query)));
  }
  const sortColumn = state.sort ? byId.get(state.sort.id) : undefined;
  if (sortColumn?.value) {
    const dir = state.sort!.desc ? -1 : 1;
    const get = sortColumn.value;
    // Stable: equal keys keep their incoming order.
    rows = rows
      .map((row, i) => ({ row, i, v: get(row) }))
      .sort((a, b) => {
        const c = compareValues(a.v, b.v);
        // Empty values stay last in both directions.
        const emptyA = a.v == null || a.v === '';
        const emptyB = b.v == null || b.v === '';
        if (emptyA !== emptyB) return emptyA ? 1 : -1;
        return c * dir || a.i - b.i;
      })
      .map((x) => x.row);
  }
  return rows;
}

/** Cycle a header's sort: ascending → descending → off. */
export function nextSort(current: SortState | null, id: string): SortState | null {
  if (current?.id !== id) return { id, desc: false };
  if (!current.desc) return { id, desc: true };
  return null;
}

export type FlatItem<T> =
  | { kind: 'group'; key: string; label: string; rows: T[] }
  | { kind: 'row'; row: T; index: number };

/** Rows (already processed) interleaved with group headers. `index` counts data rows only. */
export function flattenGroups<T>(rows: readonly T[], group: GroupOption<T> | undefined): FlatItem<T>[] {
  if (!group) return rows.map((row, index) => ({ kind: 'row', row, index }));
  const buckets = new Map<string, T[]>();
  for (const row of rows) {
    const key = group.key(row) || '—';
    const bucket = buckets.get(key);
    if (bucket) bucket.push(row);
    else buckets.set(key, [row]);
  }
  const keys = [...buckets.keys()].sort(group.compare ?? ((a, b) => collator.compare(a, b)));
  const out: FlatItem<T>[] = [];
  let index = 0;
  for (const key of keys) {
    const groupRows = buckets.get(key)!;
    out.push({ kind: 'group', key, label: key, rows: groupRows });
    for (const row of groupRows) out.push({ kind: 'row', row, index: index++ });
  }
  return out;
}

// ── Saved views (localStorage, per user and table) ───────────────────────

export interface SavedView {
  id: string;
  label: string;
  state: TableState;
}

export const savedViewsKey = (storageKey: string) => `nm-table-views:${storageKey}`;

export function loadSavedViews(storageKey: string): SavedView[] {
  try {
    const raw = window.localStorage.getItem(savedViewsKey(storageKey));
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((v): v is SavedView => Boolean(v) && typeof (v as SavedView).id === 'string' && typeof (v as SavedView).label === 'string')
      .map((v) => ({ ...v, state: normaliseState(v.state) }));
  } catch {
    return [];
  }
}

export function storeSavedViews(storageKey: string, views: SavedView[]) {
  try {
    window.localStorage.setItem(savedViewsKey(storageKey), JSON.stringify(views));
  } catch {
    // Storage blocked: the view lasts for this session only.
  }
}
