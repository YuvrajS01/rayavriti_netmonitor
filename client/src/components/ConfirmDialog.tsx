import { useRef } from 'react';
import Dialog from './ui/Dialog';
import Button from './ui/Button';

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

/**
 * Confirmation on the shared <Dialog>. Focus starts on Cancel so a stray
 * Enter can never trigger the destructive action; the confirm button is
 * solid (danger or primary) so it reads as the consequential choice.
 */
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
  const cancelRef = useRef<HTMLButtonElement>(null);
  return (
    <Dialog
      open={open}
      onClose={onCancel}
      role="alertdialog"
      size="sm"
      title={title}
      initialFocusRef={cancelRef}
      footer={
        <>
          <Button ref={cancelRef} variant="secondary" onClick={onCancel}>{cancelLabel}</Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm}>{confirmLabel}</Button>
        </>
      }
    >
      <p className="text-sm leading-relaxed text-fg-muted">{message}</p>
    </Dialog>
  );
}
