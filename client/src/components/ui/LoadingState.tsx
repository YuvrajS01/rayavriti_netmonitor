export default function LoadingState({ message = 'Loading...' }: { message?: string }) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-3">
      <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
      <p className="text-xs text-on-surface-variant uppercase tracking-widest">{message}</p>
    </div>
  );
}
