import { act, render, screen, within, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DataTable, DataTableToolbar } from '../DataTable';
import { useDataTable } from '../useDataTable';
import { flattenGroups, loadSavedViews, nextSort, processRows, type Column, type TableView } from '../dataTableModel';

interface Row { id: number; name: string; status: string; rt: number | null }

const ROWS: Row[] = [
  { id: 1, name: 'core-sw', status: 'up', rt: 4 },
  { id: 2, name: 'edge-fw', status: 'down', rt: null },
  { id: 3, name: 'cam-12', status: 'up', rt: 40 },
  { id: 4, name: 'ap-lib', status: 'warning', rt: 12 },
];

const COLUMNS: Column<Row>[] = [
  { id: 'name', header: 'Name', value: (r) => r.name, filter: 'text', hideable: false },
  { id: 'status', header: 'Status', value: (r) => r.status, filter: [{ value: 'up', label: 'Up' }, { value: 'down', label: 'Down' }] },
  { id: 'rt', header: 'Response', value: (r) => r.rt, align: 'end' },
];

describe('dataTableModel', () => {
  const base = { sort: null, filters: {}, query: '', hidden: [], groupBy: null };

  it('filters by column, query and predicate, then sorts with empties last', () => {
    expect(processRows(ROWS, COLUMNS, { ...base, filters: { status: 'up' } }).map((r) => r.id)).toEqual([1, 3]);
    expect(processRows(ROWS, COLUMNS, { ...base, query: 'CAM' }).map((r) => r.id)).toEqual([3]);
    expect(processRows(ROWS, COLUMNS, base, (r) => r.id > 2).map((r) => r.id)).toEqual([3, 4]);
    expect(processRows(ROWS, COLUMNS, { ...base, sort: { id: 'rt', desc: false } }).map((r) => r.id)).toEqual([1, 4, 3, 2]);
    expect(processRows(ROWS, COLUMNS, { ...base, sort: { id: 'rt', desc: true } }).map((r) => r.id)).toEqual([3, 4, 1, 2]);
  });

  it('cycles sort ascending → descending → off', () => {
    expect(nextSort(null, 'a')).toEqual({ id: 'a', desc: false });
    expect(nextSort({ id: 'a', desc: false }, 'a')).toEqual({ id: 'a', desc: true });
    expect(nextSort({ id: 'a', desc: true }, 'a')).toBeNull();
  });

  it('groups rows with headers and keeps a data-row index', () => {
    const items = flattenGroups(ROWS, { id: 'status', label: 'Status', key: (r) => r.status });
    expect(items.filter((i) => i.kind === 'group').map((i) => i.kind === 'group' && i.key)).toEqual(['down', 'up', 'warning']);
    expect(items.filter((i) => i.kind === 'row').map((i) => i.kind === 'row' && i.index)).toEqual([0, 1, 2, 3]);
  });

  it('ignores corrupt saved views', () => {
    localStorage.setItem('nm-table-views:t', '{oops');
    expect(loadSavedViews('t')).toEqual([]);
  });
});

function Harness({ views, onOpen, bulk }: { views?: TableView<Row>[]; onOpen?: (r: Row) => void; bulk?: (rows: Row[]) => void }) {
  const [filters, setFilters] = useState(false);
  const table = useDataTable({ data: ROWS, columns: COLUMNS, getRowId: (r) => r.id, views, storageKey: 'test:1' });
  return (
    <>
      <DataTableToolbar table={table} searchLabel="Search rows" showFilters={filters} onToggleFilters={() => setFilters((v) => !v)} />
      <DataTable
        table={table}
        label="Rows"
        rowLabel={(r) => r.name}
        selectable
        showFilters={filters}
        onRowOpen={onOpen}
        bulkActions={(rows) => <button type="button" onClick={() => bulk?.(rows)}>Bulk go</button>}
      />
    </>
  );
}

const dataRows = () => screen.getAllByRole('row').filter((r) => r.hasAttribute('aria-selected'));

describe('<DataTable>', () => {
  beforeEach(() => localStorage.clear());

  it('sorts from the header and exposes aria-sort', async () => {
    render(<Harness />);
    const header = screen.getByRole('columnheader', { name: /Response/ });
    expect(header).toHaveAttribute('aria-sort', 'none');
    await userEvent.click(within(header).getByRole('button'));
    expect(header).toHaveAttribute('aria-sort', 'ascending');
    expect(dataRows().map((r) => r.textContent)).toEqual([expect.stringContaining('core-sw'), expect.stringContaining('ap-lib'), expect.stringContaining('cam-12'), expect.stringContaining('edge-fw')]);
  });

  it('filters per column and hides columns', async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole('button', { name: /Filters/ }));
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Filter by status' }), 'down');
    expect(dataRows()).toHaveLength(1);

    await userEvent.click(screen.getByRole('button', { name: /Columns/ }));
    await userEvent.click(screen.getByRole('checkbox', { name: 'Response' }));
    expect(screen.queryByRole('columnheader', { name: /Response/ })).toBeNull();
  });

  it('navigates rows with the keyboard, selects with Space and opens with Enter', async () => {
    const onOpen = vi.fn();
    const bulk = vi.fn();
    render(<Harness onOpen={onOpen} bulk={bulk} />);
    const grid = screen.getByRole('grid', { name: 'Rows' });
    act(() => grid.focus());
    expect(grid.getAttribute('aria-activedescendant')).toMatch(/row-1$/);
    fireEvent.keyDown(grid, { key: 'ArrowDown' });
    expect(grid.getAttribute('aria-activedescendant')).toMatch(/row-2$/);
    fireEvent.keyDown(grid, { key: ' ' });
    fireEvent.keyDown(grid, { key: 'End' });
    fireEvent.keyDown(grid, { key: ' ' });
    fireEvent.keyDown(grid, { key: 'Enter' });
    expect(onOpen).toHaveBeenCalledWith(ROWS[3]);

    const bar = screen.getByRole('region', { name: 'Bulk actions' });
    expect(bar).toHaveTextContent('2 selected');
    await userEvent.click(within(bar).getByRole('button', { name: 'Bulk go' }));
    expect((bulk.mock.calls[0]![0] as Row[]).map((r) => r.id)).toEqual([2, 4]);
  });

  it('selects every visible row from the header checkbox', async () => {
    render(<Harness />);
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search rows' }), 'e');
    await userEvent.click(screen.getByRole('checkbox', { name: /Select all 2 rows/ }));
    expect(screen.getByRole('region', { name: 'Bulk actions' })).toHaveTextContent('2 selected');
  });

  it('saves a view to localStorage and applies built-in views', async () => {
    const views: TableView<Row>[] = [
      { id: 'all', label: 'All rows', state: {}, builtIn: true },
      { id: 'cams', label: 'Cameras', state: {}, predicate: (r) => r.name.startsWith('cam'), builtIn: true },
    ];
    render(<Harness views={views} />);
    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'View' }), 'cams');
    expect(dataRows()).toHaveLength(1);

    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'View' }), 'all');
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search rows' }), 'sw');
    await userEvent.click(screen.getByRole('button', { name: 'Save view' }));
    await userEvent.type(screen.getByLabelText('Name'), 'Switches');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    const stored = loadSavedViews('test:1');
    expect(stored).toHaveLength(1);
    expect(stored[0]!.label).toBe('Switches');
    expect(stored[0]!.state.query).toBe('sw');
    expect(screen.getByRole('combobox', { name: 'View' })).toHaveDisplayValue('Switches');
  });
});
