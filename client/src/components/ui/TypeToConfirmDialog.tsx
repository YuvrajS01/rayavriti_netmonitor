import { useId, useRef, useState, type ReactNode } from 'react';
import Dialog from './Dialog';
import Button from './Button';

interface TypeToConfirmDialogProps {
  open: boolean;
  title: string;
  message: ReactNode;
  /** The exact text the operator must type (a device name, "delete 12"). */
  phrase: string;
  confirmLabel: string;
  busy?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Destructive confirmation that needs the operator to type a phrase, for
 * actions that are hard to undo (deleting a device and its history, bulk
 * deletes). Focus starts on the input; the button stays disabled until the
 * phrase matches exactly.
 */
export default function TypeToConfirmDialog({ open, title, message, phrase, confirmLabel, busy = false, onConfirm, onCancel }: TypeToConfirmDialogProps) {
  const [typed, setTyped] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const inputId = useId();
  const matches = typed.trim() === phrase;
  const close = () => {
    setTyped('');
    onCancel();
  };
  return (
    <Dialog
      open={open}
      onClose={close}
      role="alertdialog"
      size="sm"
      title={title}
      initialFocusRef={inputRef}
      closeOnBackdrop={false}
      footer={
        <>
          <Button variant="secondary" onClick={close}>Cancel</Button>
          <Button variant="danger" type="submit" form={`${inputId}-form`} disabled={!matches || busy}>
            {busy ? 'Working…' : confirmLabel}
          </Button>
        </>
      }
    >
      <form
        id={`${inputId}-form`}
        onSubmit={(e) => {
          e.preventDefault();
          if (matches && !busy) onConfirm();
        }}
        className="space-y-4"
      >
        <div className="text-sm leading-relaxed text-fg-muted">{message}</div>
        <div>
          <label htmlFor={inputId} className="mb-1.5 block text-sm text-fg">
            Type <span className="rounded-sm bg-surface-3 px-1 font-mono text-fg">{phrase}</span> to confirm
          </label>
          <input
            ref={inputRef}
            id={inputId}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            aria-invalid={typed !== '' && !matches}
            className="h-control w-full rounded-md border border-line bg-inset px-3 font-mono text-sm text-fg outline-none focus:border-primary aria-[invalid=true]:border-error"
          />
        </div>
      </form>
    </Dialog>
  );
}
