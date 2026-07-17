import { useId, useMemo } from 'react';

interface Props {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
  showTrend?: boolean;
  className?: string;
}

export default function Sparkline({ data, width = 92, height = 28, color = 'var(--color-success)', showTrend = false, className }: Props) {
  const id = useId().replace(/:/g, '');
  const { line, area, trend } = useMemo(() => {
    const values = data.length > 1 ? data : [...data, ...(data.length ? data : [0])];
    const min = Math.min(...values), max = Math.max(...values), range = max - min || 1;
    const points = values.map((value, index) => {
      const x = (index / Math.max(values.length - 1, 1)) * width;
      const y = height - 3 - ((value - min) / range) * (height - 7);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    return { line: points.join(' '), area: `0,${height} ${points.join(' ')} ${width},${height}`, trend: values.at(-1)! - values[0] };
  }, [data, height, width]);
  if (!data.length) return null;
  return <div className={`inline-flex items-center gap-1 ${className ?? ''}`} aria-label={`Trend ${trend >= 0 ? 'up' : 'down'}`}>
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Recent trend">
      <defs><linearGradient id={`spark-${id}`} x1="0" x2="0" y1="0" y2="1"><stop stopColor={color} stopOpacity=".4"/><stop offset="1" stopColor={color} stopOpacity="0"/></linearGradient></defs>
      <polygon points={area} fill={`url(#spark-${id})`} />
      <polyline points={line} fill="none" stroke={color} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
    {showTrend && <span className={`material-symbols-outlined text-sm ${trend >= 0 ? 'text-success' : 'text-error'}`}>{trend >= 0 ? 'trending_up' : 'trending_down'}</span>}
  </div>;
}
