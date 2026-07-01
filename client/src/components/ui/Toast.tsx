import { useState, useCallback, useRef, type ReactNode } from 'react';
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

function ToastContainer({ toasts, onRemove }: { toasts: Toast[]; onRemove: (id: number) => void }) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-20 right-4 z-[100] flex flex-col gap-2 pointer-events-none" aria-live="polite" aria-label="Notifications">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role="status"
          className={`pointer-events-auto max-w-sm px-4 py-3 rounded-lg border shadow-lg flex items-center gap-3 animate-slide-up ${
            toast.type === 'success'
              ? 'bg-success/10 border-success/30 text-success'
              : toast.type === 'error'
                ? 'bg-error/10 border-error/30 text-error'
                : 'bg-surface-container-high border-outline-variant/30 text-on-surface'
          }`}
        >
          <span className="material-symbols-outlined text-lg">
            {toast.type === 'success' ? 'check_circle' : toast.type === 'error' ? 'error' : 'info'}
          </span>
          <span className="text-sm font-medium flex-1">{toast.message}</span>
          <button onClick={() => onRemove(toast.id)} className="material-symbols-outlined text-sm opacity-60 hover:opacity-100" aria-label="Dismiss">
            close
          </button>
        </div>
      ))}
    </div>
  );
}
