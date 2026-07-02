interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
}

export default function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div role="alert" className="bg-error/10 border border-error/30 rounded-lg p-6 text-center">
      <span className="material-symbols-outlined text-error text-3xl mb-2">error</span>
      <p className="text-sm text-error font-medium">{message}</p>
      {onRetry && (
        <button onClick={onRetry} className="mt-3 text-xs text-on-surface-variant hover:text-primary transition-colors underline">
          Retry
        </button>
      )}
    </div>
  );
}
