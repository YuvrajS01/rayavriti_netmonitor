import { useEffect, useRef, useState } from 'react';

export type CounterFormat = 'number' | 'percent' | 'bytes' | 'ms';

interface Props {
  value: number;
  format?: CounterFormat;
  duration?: number;
  className?: string;
  decimals?: number;
}

function formatValue(value: number, format: CounterFormat, decimals: number) {
  if (format === 'bytes') {
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let unit = 0;
    let next = value;
    while (next >= 1024 && unit < units.length - 1) { next /= 1024; unit++; }
    return `${next.toFixed(unit ? Math.min(decimals, 1) : 0)} ${units[unit]}`;
  }
  if (format === 'percent') return `${value.toFixed(decimals)}%`;
  if (format === 'ms') return `${value.toFixed(decimals)}ms`;
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: decimals }).format(value);
}

export default function AnimatedCounter({ value, format = 'number', duration = 650, className, decimals = 0 }: Props) {
  const [display, setDisplay] = useState(value);
  const previous = useRef(value);

  useEffect(() => {
    const start = previous.current;
    const startAt = performance.now();
    let frame = 0;
    const tick = (now: number) => {
      const progress = Math.min(1, (now - startAt) / duration);
      const eased = 1 - Math.pow(1 - progress, 4);
      setDisplay(start + (value - start) * eased);
      if (progress < 1) frame = requestAnimationFrame(tick);
      else previous.current = value;
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [value, duration]);

  return <span className={className}>{formatValue(display, format, decimals)}</span>;
}
