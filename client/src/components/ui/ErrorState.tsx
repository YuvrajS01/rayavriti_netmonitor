import Button from './Button';

interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
}

export default function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div role="alert" className="bg-surface-1 border border-line border-l-4 border-l-error rounded-lg p-6 text-center">
      <span className="material-symbols-outlined text-error text-3xl mb-2" aria-hidden="true">error</span>
      <p className="text-sm text-fg font-medium">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" icon="refresh" onClick={onRetry} className="mt-4">
          Retry
        </Button>
      )}
    </div>
  );
}
