interface SortHeaderProps<K extends string> {
  label: string;
  sortKey: K;
  activeKey: K;
  desc: boolean;
  onSort: (key: K) => void;
  align?: 'left' | 'center' | 'right';
}

/** A sortable column header: a real button inside the <th>, with aria-sort. */
export default function SortHeader<K extends string>({ label, sortKey, activeKey, desc, onSort, align = 'left' }: SortHeaderProps<K>) {
  const active = sortKey === activeKey;
  const justify = align === 'right' ? 'justify-end' : align === 'center' ? 'justify-center' : 'justify-start';
  return (
    <th scope="col" aria-sort={active ? (desc ? 'descending' : 'ascending') : 'none'} className="pb-3 font-medium">
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className={`inline-flex w-full items-center gap-1 rounded-sm ${justify} ${active ? 'text-fg' : 'hover:text-fg'} transition-colors`}
      >
        {label}
        <span className="material-symbols-outlined text-[16px]" aria-hidden="true">
          {active ? (desc ? 'arrow_downward' : 'arrow_upward') : 'unfold_more'}
        </span>
      </button>
    </th>
  );
}
