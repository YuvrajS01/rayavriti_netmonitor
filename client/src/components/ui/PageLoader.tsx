export default function PageLoader() {
  return (
    <div role="status" aria-live="polite" className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3">
        <span className="material-symbols-outlined animate-spin text-3xl text-fg-muted" aria-hidden="true">progress_activity</span>
        <p className="text-sm text-fg-muted">Loading…</p>
      </div>
    </div>
  );
}
