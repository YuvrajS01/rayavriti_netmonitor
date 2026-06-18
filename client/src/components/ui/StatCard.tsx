interface StatCardProps {
  label: string;
  value: string | number;
  color?: string;
  icon?: string;
}

export default function StatCard({ label, value, color = 'text-primary', icon }: StatCardProps) {
  return (
    <div className="bg-surface-container-low p-6 rounded-lg border border-outline-variant/20">
      <div className="flex items-center gap-2 mb-1">
        {icon && <span className="material-symbols-outlined text-sm opacity-60">{icon}</span>}
        <p className="text-on-surface-variant text-xs uppercase tracking-wide">{label}</p>
      </div>
      <p className={`font-headline text-2xl font-semibold ${color}`}>{value}</p>
    </div>
  );
}
