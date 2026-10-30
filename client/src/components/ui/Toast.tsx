import { useState, useCallback, useLayoutEffect, useRef, type ReactNode } from 'react';
import { StatusShapeIcon } from './Status';
import { STATUS_META } from '../../utils/status';
import { ToastContext, type ToastContextValue } from './useToast';
import type { Toast } from './useToast';

export type { Toast };

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextIdRef = useRef(0);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const addToast = useCallback((message: string, type: Toast['type'] = 'info') => {
    const id = ++nextIdRef.current;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => removeToast(id), 4000);
  }, [removeToast]);

  const value: ToastContextValue = { toasts, addToast, removeToast };

  return (
    <ToastContext.Provider value={value}>
      {children}
      <ToastContainer toasts={toasts} onRemove={removeToast} />
    </ToastContext.Provider>
  );
}

const TOAST_STATUS = { success: 'up', error: 'down', info: 'info' } as const;

function ToastContainer({ toasts, onRemove }: { toasts: Toast[]; onRemove: (id: number) => void }) {
  const ref = useRef<HTMLDivElement>(null);

  // Toasts live in a manual popover so they render in the top layer, above
  // an open modal <dialog> (which would otherwise cover them with its
  // backdrop). Re-showing moves the popover to the top of the stack.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || typeof el.showPopover !== 'function') return;
    if (el.matches(':popover-open')) el.hidePopover();
    if (toasts.length > 0) el.showPopover();
  }, [toasts]);

  return (
    <div
      ref={ref}
      popover="manual"
      className="fixed inset-auto bottom-20 right-4 z-toast m-0 flex flex-col gap-2 overflow-visible border-0 bg-transparent p-0 lg:bottom-4"
      aria-live="polite"
      aria-label="Notifications"
    >
      {toasts.map((toast) => {
        const meta = STATUS_META[TOAST_STATUS[toast.type]];
        return (
          <div
            key={toast.id}
            role="status"
            className="pointer-events-auto flex max-w-sm items-center gap-3 rounded-md border border-line-strong bg-surface-2 px-4 py-3 text-fg shadow-e2 animate-slide-up"
          >
            <span className={meta.text}><StatusShapeIcon shape={meta.shape} size={12} /></span>
            <span className="flex-1 text-sm font-medium">{toast.message}</span>
            <button
              type="button"
              onClick={() => onRemove(toast.id)}
              className="material-symbols-outlined rounded text-[18px] text-fg-muted hover:text-fg"
              aria-label="Dismiss notification"
            >
              close
            </button>
          </div>
        );
      })}
    </div>
  );
}
