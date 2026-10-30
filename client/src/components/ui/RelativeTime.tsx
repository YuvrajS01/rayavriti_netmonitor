import { formatAbsolute, formatRelative, useNow } from '../../utils/time';

/** Relative time with the absolute timestamp as tooltip and machine-readable dateTime. */
export default function RelativeTime({ value, className = '' }: { value: string | number | Date | null | undefined; className?: string }) {
  const current = useNow();
  if (value == null || value === '') return <span className={className}>—</span>;
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return <span className={className}>—</span>;
  const absolute = formatAbsolute(date);
  return (
    <time dateTime={date.toISOString()} title={absolute} className={className}>
      {formatRelative(date, current)}
    </time>
  );
}
