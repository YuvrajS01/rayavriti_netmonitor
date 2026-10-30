import { useEffect, useRef } from 'react';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const confirmBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;

    previousFocus.current = document.activeElement as HTMLElement;
    setTimeout(() => confirmBtnRef.current?.focus(), 0);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCancel();
        return;
      }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocus.current?.focus();
    };
  }, [open, onCancel]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center p-4 pt-20 bg-black/60" onClick={onCancel}>
      <div
        ref={dialogRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby="confirm-message"
        className="bg-surface-1 border border-line rounded-lg w-full max-w-md max-h-[90vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-6 shrink-0">
          <div className="flex items-center gap-3 mb-4">
            <span className={`material-symbols-outlined text-3xl ${danger ? 'text-error' : 'text-primary'}`}>
              {danger ? 'warning' : 'help'}
            </span>
            <h2 id="confirm-title" className="font-sans text-lg font-semibold text-fg uppercase tracking-tight">
              {title}
            </h2>
          </div>
          <p id="confirm-message" className="text-sm text-fg-muted leading-relaxed">
            {message}
          </p>
        </div>
        <div className="flex border-t border-line shrink-0">
          <button
            onClick={onCancel}
            className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-fg-muted hover:bg-surface-1 transition-colors"
          >
            {cancelLabel}
          </button>
          <div className="w-px bg-line/20" />
          <button
            ref={confirmBtnRef}
            onClick={onConfirm}
            className={`flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide transition-colors ${
              danger
                ? 'text-error hover:bg-error/10'
                : 'text-primary hover:bg-primary/10'
            }`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
