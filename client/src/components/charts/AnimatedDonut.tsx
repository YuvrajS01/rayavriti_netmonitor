interface Slice { label: string; value: number; color: string; }
export default function AnimatedDonut({ slices, label = 'Total' }: { slices: Slice[]; label?: string }) {
  const total = slices.reduce((sum, slice) => sum + slice.value, 0) || 1;
  const segments = slices.map((slice, index) => ({ slice, length: slice.value / total * 100, offset: slices.slice(0, index).reduce((sum, prior) => sum + prior.value / total * 100, 0) }));
  return <div className="relative grid place-items-center w-40 h-40" role="img" aria-label={`${label}: ${total}`}><svg viewBox="0 0 42 42" className="w-full h-full -rotate-90">{segments.map(({ slice, length, offset }) => <circle key={slice.label} cx="21" cy="21" r="15.9" fill="none" stroke={slice.color} strokeWidth="6" strokeDasharray={`${length} ${100 - length}`} strokeDashoffset={-offset} className="gauge-ring" />)}</svg><span className="absolute text-center"><b className="block font-headline text-xl">{total}</b><small className="text-[9px] uppercase tracking-wide text-on-surface-variant">{label}</small></span></div>;
}
