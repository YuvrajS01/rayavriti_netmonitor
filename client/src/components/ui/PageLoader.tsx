export default function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
        <p className="text-xs text-on-surface-variant uppercase tracking-wide">Loading...</p>
      </div>
    </div>
  );
}
