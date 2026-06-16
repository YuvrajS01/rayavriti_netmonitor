interface ChartDataTableProps {
  title: string;
  columns: string[];
  rows: (string | number)[][];
}

export default function ChartDataTable({ title, columns, rows }: ChartDataTableProps) {
  if (rows.length === 0) return null;

  return (
    <table className="w-full text-left border-collapse text-xs" role="table" aria-label={title}>
      <thead>
        <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
          {columns.map((col) => (
            <th key={col} className="pb-2 font-medium pr-4">{col}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i} className="border-b border-outline-variant/10">
            {row.map((cell, j) => (
              <td key={j} className="py-1.5 pr-4 font-mono">{cell}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
