export default function LoadingState({ message = 'Loading...' }: { message?: string }) {
  return (
    <div role="status" aria-live="polite" className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
      <span className="material-symbols-outlined text-3xl text-fg-muted animate-pulse">hourglass_top</span>
      <p className="text-xs text-fg-muted uppercase tracking-wide font-sans font-medium">{message}</p>
    </div>
  );
}
