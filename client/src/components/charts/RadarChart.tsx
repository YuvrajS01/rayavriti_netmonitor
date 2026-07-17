interface RadarSeries { label: string; values: number[]; color?: string; }
interface Props { axes: string[]; series: RadarSeries[]; size?: number; }
export default function RadarChart({ axes, series, size = 180 }: Props) {
  const center = size / 2, radius = size * .34;
  const point = (index: number, value = 100) => { const angle = -Math.PI / 2 + (Math.PI * 2 * index) / axes.length; const r = radius * value / 100; return `${center + Math.cos(angle) * r},${center + Math.sin(angle) * r}`; };
  return <svg viewBox={`0 0 ${size} ${size}`} className="w-full max-w-[210px] mx-auto" role="img" aria-label="Health factor radar chart">
    {[25, 50, 75, 100].map(level => <polygon key={level} points={axes.map((_, i) => point(i, level)).join(' ')} fill="none" stroke="var(--color-outline-variant)" strokeWidth=".6" opacity={level === 100 ? .8 : .45} />)}
    {axes.map((axis, i) => <g key={axis}><line x1={center} y1={center} x2={point(i).split(',')[0]} y2={point(i).split(',')[1]} stroke="var(--color-outline-variant)" strokeWidth=".6"/><text x={point(i, 122).split(',')[0]} y={point(i, 122).split(',')[1]} textAnchor="middle" dominantBaseline="middle" fill="var(--color-on-surface-variant)" fontSize="8">{axis}</text></g>)}
    {series.map((item, i) => <polygon key={item.label} points={item.values.map((v, index) => point(index, v)).join(' ')} fill={item.color || (i ? 'var(--color-chart-3)' : 'var(--color-success)')} fillOpacity=".18" stroke={item.color || (i ? 'var(--color-chart-3)' : 'var(--color-success)')} strokeWidth="2" className="radar-polygon" />)}
  </svg>;
}
