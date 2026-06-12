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
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
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
    <div className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm" onClick={onCancel}>
      <div
        ref={dialogRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        aria-describedby="confirm-message"
        className="bg-surface-container-low border border-outline-variant/30 rounded-xl w-full max-w-md overflow-hidden shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-6">
          <div className="flex items-center gap-3 mb-4">
            <span className={`material-symbols-outlined text-3xl ${danger ? 'text-error' : 'text-primary'}`}>
              {danger ? 'warning' : 'help'}
            </span>
            <h2 id="confirm-title" className="font-headline text-lg font-bold text-on-surface uppercase tracking-tight">
              {title}
            </h2>
          </div>
          <p id="confirm-message" className="text-sm text-on-surface-variant leading-relaxed">
            {message}
          </p>
        </div>
        <div className="flex border-t border-outline-variant/20">
          <button
            onClick={onCancel}
            className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-widest text-on-surface-variant hover:bg-surface-container-high transition-colors"
          >
            {cancelLabel}
          </button>
          <div className="w-px bg-outline-variant/20" />
          <button
            ref={confirmBtnRef}
            onClick={onConfirm}
            className={`flex-1 py-3 text-xs font-headline font-bold uppercase tracking-widest transition-colors ${
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
