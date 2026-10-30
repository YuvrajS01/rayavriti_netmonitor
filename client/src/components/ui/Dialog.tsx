import { useEffect, useId, useLayoutEffect, useRef, type ReactNode, type RefObject } from 'react';
import IconButton from './IconButton';

export type DialogSize = 'sm' | 'md' | 'lg' | 'xl' | '2xl';

interface DialogProps {
  open: boolean;
  /** Called for Esc, the close button and a backdrop click. */
  onClose: () => void;
  /** Visible title; also the dialog's accessible name. */
  title: ReactNode;
  /** Optional line under the title; becomes aria-describedby. */
  description?: ReactNode;
  children?: ReactNode;
  /** Action row pinned to the bottom (buttons, right-aligned). */
  footer?: ReactNode;
  size?: DialogSize;
  /** `drawer` slides in from the right edge (detail panels). */
  placement?: 'center' | 'drawer';
  /** `alertdialog` for confirmations that interrupt the user. */
  role?: 'dialog' | 'alertdialog';
  /** Element to focus on open; defaults to the first focusable control in the body. */
  initialFocusRef?: RefObject<HTMLElement | null>;
  /** Extra content in the header, before the close button (badges, actions). */
  headerActions?: ReactNode;
  /** Set false for dialogs that must not close on backdrop click (unsaved forms). */
  closeOnBackdrop?: boolean;
  /** Classes for the body wrapper (padding, grid...). */
  bodyClassName?: string;
}

const SIZE: Record<DialogSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
  '2xl': 'max-w-6xl',
};

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Modal dialog on the native <dialog> element.
 *
 * showModal() gives us, for free: the top layer (no z-index fights with the
 * header or the mobile nav), an inert background, a focus trap and Esc
 * handling. We add the labelling, focus placement, focus return to the
 * opener, and backdrop-click to close.
 */
export default function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
  placement = 'center',
  role = 'dialog',
  initialFocusRef,
  headerActions,
  closeOnBackdrop = true,
  bodyClassName = 'px-6 py-5',
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descriptionId = useId();
  const onCloseRef = useRef(onClose);
  useLayoutEffect(() => { onCloseRef.current = onClose; });

  useLayoutEffect(() => {
    const dialog = ref.current;
    if (!open || !dialog) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (!dialog.open) {
      if (typeof dialog.showModal === 'function') dialog.showModal();
      else dialog.setAttribute('open', '');
    }
    const target = initialFocusRef?.current ?? bodyRef.current?.querySelector<HTMLElement>(FOCUSABLE) ?? dialog.querySelector<HTMLElement>(FOCUSABLE);
    target?.focus();
    return () => {
      if (dialog.open && typeof dialog.close === 'function') dialog.close();
      // Return focus to whatever opened the dialog (if it is still there).
      if (opener?.isConnected) opener.focus();
    };
  }, [open, initialFocusRef]);

  // Esc fires `cancel`; route it through onClose so React state stays the source of truth.
  useEffect(() => {
    const dialog = ref.current;
    if (!open || !dialog) return;
    const onCancel = (event: Event) => {
      event.preventDefault();
      onCloseRef.current();
    };
    dialog.addEventListener('cancel', onCancel);
    return () => dialog.removeEventListener('cancel', onCancel);
  }, [open]);

  if (!open) return null;

  const drawer = placement === 'drawer';
  return (
    <dialog
      ref={ref}
      role={role === 'alertdialog' ? 'alertdialog' : undefined}
      aria-modal="true"
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      onKeyDown={(event) => {
        // jsdom / browsers without native cancel: keep Esc working.
        if (event.key === 'Escape' && typeof ref.current?.showModal !== 'function') {
          event.preventDefault();
          onCloseRef.current();
        }
      }}
      onMouseDown={(event) => {
        // A press that starts on the <dialog> itself (not the panel) is on the backdrop.
        if (closeOnBackdrop && event.target === ref.current) onCloseRef.current();
      }}
      className={`bg-transparent p-0 text-fg backdrop:bg-scrim ${
        drawer
          ? 'm-0 ml-auto h-dvh max-h-dvh w-full max-w-lg'
          : `m-auto w-[calc(100%-2rem)] ${SIZE[size]} max-h-[calc(100dvh-2rem)]`
      }`}
    >
      <div
        className={`flex flex-col border border-line bg-surface-1 shadow-e3 ${
          drawer ? 'h-full border-y-0 border-r-0' : 'max-h-[calc(100dvh-2rem)] rounded-lg'
        }`}
      >
        <header className="flex items-start gap-3 border-b border-line px-6 py-4">
          <div className="min-w-0 flex-1">
            <h2 id={titleId} className="text-lg font-semibold text-fg truncate">{title}</h2>
            {description && <p id={descriptionId} className="mt-0.5 text-sm text-fg-muted">{description}</p>}
          </div>
          {headerActions}
          <IconButton icon="close" label="Close dialog" onClick={() => onCloseRef.current()} className="-mr-2 -mt-1" />
        </header>
        <div ref={bodyRef} className={`min-h-0 flex-1 overflow-y-auto ${bodyClassName}`}>
          {children}
        </div>
        {footer && <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-line px-6 py-4">{footer}</footer>}
      </div>
    </dialog>
  );
}
