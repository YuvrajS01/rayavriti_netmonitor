import { useRef, type KeyboardEvent, type ReactNode } from 'react';

export interface TabItem<T extends string = string> {
  id: T;
  label: ReactNode;
  icon?: string;
  /** Small count shown after the label. */
  badge?: number;
}

interface TabsProps<T extends string> {
  items: readonly TabItem<T>[];
  value: T;
  onChange: (value: T) => void;
  /** Accessible name of the tab list ("Report sections"). */
  label: string;
  /** Prefix shared with <TabPanel idBase> to link tabs and panels. */
  idBase: string;
  className?: string;
}

/**
 * WAI-ARIA tabs with automatic activation: arrow keys move and select,
 * Home/End jump, only the selected tab is in the tab order.
 */
export function Tabs<T extends string>({ items, value, onChange, label, idBase, className = '' }: TabsProps<T>) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const current = items.findIndex((item) => item.id === value);
    let next = -1;
    if (event.key === 'ArrowRight') next = (current + 1) % items.length;
    else if (event.key === 'ArrowLeft') next = (current - 1 + items.length) % items.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = items.length - 1;
    if (next < 0) return;
    event.preventDefault();
    const item = items[next];
    if (!item) return;
    onChange(item.id);
    refs.current[next]?.focus();
  };

  return (
    <div
      role="tablist"
      aria-label={label}
      onKeyDown={onKeyDown}
      className={`flex gap-1 overflow-x-auto border-b border-line ${className}`}
    >
      {items.map((item, index) => {
        const selected = item.id === value;
        return (
          <button
            key={item.id}
            ref={(el) => { refs.current[index] = el; }}
            type="button"
            role="tab"
            id={`${idBase}-tab-${item.id}`}
            aria-selected={selected}
            aria-controls={`${idBase}-panel-${item.id}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(item.id)}
            className={`relative -mb-px inline-flex h-10 shrink-0 items-center gap-2 border-b-2 px-3 text-sm font-medium whitespace-nowrap transition-colors duration-[var(--duration-fast)] ${
              selected ? 'border-primary text-fg' : 'border-transparent text-fg-muted hover:text-fg hover:border-line-strong'
            }`}
          >
            {item.icon && <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{item.icon}</span>}
            {item.label}
            {item.badge != null && item.badge > 0 && (
              <span className="rounded-full bg-surface-3 px-1.5 text-xs tabular-nums text-fg-muted">{item.badge}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

interface TabPanelProps {
  idBase: string;
  id: string;
  /** The active tab; the panel renders only when it matches. */
  value: string;
  children: ReactNode;
  className?: string;
}

export function TabPanel({ idBase, id, value, children, className = '' }: TabPanelProps) {
  if (id !== value) return null;
  return (
    <div
      role="tabpanel"
      id={`${idBase}-panel-${id}`}
      aria-labelledby={`${idBase}-tab-${id}`}
      tabIndex={0}
      className={`focus-visible:outline-offset-4 ${className}`}
    >
      {children}
    </div>
  );
}
