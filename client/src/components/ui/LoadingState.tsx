export default function LoadingState({ message = 'Loading...' }: { message?: string }) {
  return (
    <div role="status" aria-live="polite" className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
      <span className="material-symbols-outlined animate-spin text-3xl text-fg-muted" aria-hidden="true">progress_activity</span>
      <p className="text-sm text-fg-muted">{message}</p>
    </div>
  );
}
