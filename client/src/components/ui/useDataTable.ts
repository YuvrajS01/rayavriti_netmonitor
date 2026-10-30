import { useCallback, useMemo, useState } from 'react';
import {
  flattenGroups,
  loadSavedViews,
  normaliseState,
  processRows,
  sameState,
  storeSavedViews,
  type Column,
  type FlatItem,
  type GroupOption,
  type SavedView,
  type TableState,
  type TableView,
} from './dataTableModel';

export type RowId = string | number;

export interface UseDataTableOptions<T> {
  data: readonly T[];
  columns: Column<T>[];
  getRowId: (row: T) => RowId;
  /** Built-in views; the first is the default. */
  views?: TableView<T>[];
  /** Controlled current view (e.g. from `?view=`). */
  viewId?: string | null;
  onViewChange?: (id: string) => void;
  /** Enables saved views in localStorage under this key (include the user id). */
  storageKey?: string;
  groups?: GroupOption<T>[];
}

const DEFAULT_VIEW_ID = 'all';

export interface DataTableModel<T> {
  columns: Column<T>[];
  visibleColumns: Column<T>[];
  state: TableState;
  setState: (patch: Partial<TableState>) => void;
  rows: T[];
  items: FlatItem<T>[];
  groups: GroupOption<T>[];
  views: Array<{ id: string; label: string; builtIn: boolean }>;
  viewId: string;
  setView: (id: string) => void;
  /** Current state differs from the selected view's. */
  dirty: boolean;
  canSaveViews: boolean;
  saveView: (label: string) => void;
  deleteView: (id: string) => void;
  getRowId: (row: T) => RowId;
  selected: ReadonlySet<RowId>;
  /** Selected rows that are still visible after filtering. */
  selectedRows: T[];
  isSelected: (row: T) => boolean;
  toggleRow: (row: T) => void;
  setRowsSelected: (rows: readonly T[], on: boolean) => void;
  clearSelection: () => void;
}

export function useDataTable<T>({
  data,
  columns,
  getRowId,
  views: builtInViews,
  viewId: controlledViewId,
  onViewChange,
  storageKey,
  groups = [],
}: UseDataTableOptions<T>): DataTableModel<T> {
  const baseViews = useMemo<TableView<T>[]>(
    () => (builtInViews?.length ? builtInViews : [{ id: DEFAULT_VIEW_ID, label: 'All', state: {}, builtIn: true }]),
    [builtInViews],
  );
  const [saved, setSaved] = useState<SavedView[]>(() => (storageKey ? loadSavedViews(storageKey) : []));
  const [internalViewId, setInternalViewId] = useState<string>(baseViews[0]!.id);
  const requested = controlledViewId ?? internalViewId;

  const findView = useCallback(
    (id: string): TableView<T> | undefined =>
      baseViews.find((v) => v.id === id) ?? saved.map((v) => ({ ...v, builtIn: false })).find((v) => v.id === id),
    [baseViews, saved],
  );
  const view = findView(requested) ?? baseViews[0]!;

  // State resets whenever the view changes (including via the URL).
  const [state, setFullState] = useState<TableState>(() => normaliseState(view.state));
  const [stateViewId, setStateViewId] = useState(view.id);
  if (stateViewId !== view.id) {
    setStateViewId(view.id);
    setFullState(normaliseState(view.state));
  }

  const setState = useCallback((patch: Partial<TableState>) => setFullState((prev) => ({ ...prev, ...patch })), []);

  const setView = useCallback(
    (id: string) => {
      if (onViewChange) onViewChange(id);
      else setInternalViewId(id);
    },
    [onViewChange],
  );

  const saveView = useCallback(
    (label: string) => {
      if (!storageKey) return;
      const id = `saved-${Date.now().toString(36)}`;
      setSaved((prev) => {
        const next = [...prev, { id, label: label.trim() || 'Untitled view', state }];
        storeSavedViews(storageKey, next);
        return next;
      });
      // Adopt it as the current view without resetting the state.
      setStateViewId(id);
      setView(id);
    },
    [storageKey, state, setView],
  );

  const deleteView = useCallback(
    (id: string) => {
      if (!storageKey) return;
      setSaved((prev) => {
        const next = prev.filter((v) => v.id !== id);
        storeSavedViews(storageKey, next);
        return next;
      });
      if (id === view.id) setView(baseViews[0]!.id);
    },
    [storageKey, view.id, setView, baseViews],
  );

  const rows = useMemo(() => processRows(data, columns, state, view.predicate), [data, columns, state, view.predicate]);
  const group = groups.find((g) => g.id === state.groupBy);
  const items = useMemo(() => flattenGroups(rows, group), [rows, group]);
  const visibleColumns = useMemo(() => columns.filter((c) => !state.hidden.includes(c.id)), [columns, state.hidden]);

  const [selected, setSelected] = useState<Set<RowId>>(() => new Set());
  const isSelected = useCallback((row: T) => selected.has(getRowId(row)), [selected, getRowId]);
  const toggleRow = useCallback(
    (row: T) =>
      setSelected((prev) => {
        const next = new Set(prev);
        const id = getRowId(row);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      }),
    [getRowId],
  );
  const setRowsSelected = useCallback(
    (target: readonly T[], on: boolean) =>
      setSelected((prev) => {
        const next = new Set(prev);
        for (const row of target) {
          if (on) next.add(getRowId(row));
          else next.delete(getRowId(row));
        }
        return next;
      }),
    [getRowId],
  );
  const clearSelection = useCallback(() => setSelected(new Set()), []);
  const selectedRows = useMemo(() => rows.filter((r) => selected.has(getRowId(r))), [rows, selected, getRowId]);

  return {
    columns,
    visibleColumns,
    state,
    setState,
    rows,
    items,
    groups,
    views: [...baseViews.map((v) => ({ id: v.id, label: v.label, builtIn: true })), ...saved.map((v) => ({ id: v.id, label: v.label, builtIn: false }))],
    viewId: view.id,
    setView,
    dirty: !sameState(state, normaliseState(view.state)),
    canSaveViews: Boolean(storageKey),
    saveView,
    deleteView,
    getRowId,
    selected,
    selectedRows,
    isSelected,
    toggleRow,
    setRowsSelected,
    clearSelection,
  };
}
