interface StatCardProps {
  label: string;
  value: string | number;
  color?: string;
  icon?: string;
  accentColor?: string;
}

export default function StatCard({ label, value, color = 'text-primary', icon, accentColor }: StatCardProps) {
  return (
    <div className="bg-surface-container-low p-6 rounded-xl border-l-2" style={accentColor ? { borderLeftColor: accentColor } : { borderColor: 'rgba(217,253,58,0.3)' }}>
      <div className="flex items-center gap-2 mb-1">
        {icon && <span className="material-symbols-outlined text-sm opacity-60">{icon}</span>}
        <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em]">{label}</p>
      </div>
      <p className={`font-headline text-3xl font-bold ${color}`}>{value}</p>
    </div>
  );
}
