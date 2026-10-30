import { useEffect, useId, useRef, useState, type ReactNode } from 'react';

interface PopoverProps {
  /** Renders the trigger; spread the props onto a <button>. */
  trigger: (props: { 'aria-expanded': boolean; 'aria-controls': string; onClick: () => void }) => ReactNode;
  children: ReactNode | ((close: () => void) => ReactNode);
  /** Accessible name of the panel. */
  label: string;
  align?: 'start' | 'end';
  className?: string;
}

/**
 * A disclosure panel anchored under its button (column picker, view menu).
 * Closes on Escape (focus returns to the button) and on a press outside.
 * Not modal: it does not trap focus, so it is not a Dialog.
 */
export default function Popover({ trigger, children, label, align = 'end', className = '' }: PopoverProps) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.stopPropagation();
      setOpen(false);
      // The trigger is the first button in the root.
      rootRef.current?.querySelector<HTMLButtonElement>('button')?.focus();
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const close = () => setOpen(false);
  return (
    <div ref={rootRef} className="relative">
      {trigger({
        'aria-expanded': open,
        'aria-controls': id,
        onClick: () => setOpen((v) => !v),
      })}
      {open && (
        <div
          id={id}
          role="group"
          aria-label={label}
          className={`absolute top-full z-drawer mt-1 min-w-56 rounded-lg border border-line-strong bg-surface-1 p-2 shadow-e2 ${align === 'end' ? 'right-0' : 'left-0'} ${className}`}
        >
          {typeof children === 'function' ? children(close) : children}
        </div>
      )}
    </div>
  );
}
