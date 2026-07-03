import type { ISPReportLink } from '../../api/reports';
import { useState } from 'react';

type SortKey = 'name' | 'provider' | 'uptimePercent' | 'avgLatency' | 'avgPacketLoss' | 'avgDownload' | 'avgUpload';

function badge(uptime: number) {
  if (uptime >= 99) return 'bg-primary/15 text-primary border-primary/30';
  if (uptime >= 95) return 'bg-warning/15 text-warning border-warning/30';
  return 'bg-error/15 text-error border-error/30';
}

export default function IspTab({ links }: { links: ISPReportLink[] }) {
  const [sortKey, setSortKey] = useState<SortKey>('uptimePercent');
  const [sortDesc, setSortDesc] = useState(true);

  const toggle = (key: SortKey) => {
    if (sortKey === key) setSortDesc(!sortDesc);
    else { setSortKey(key); setSortDesc(key === 'uptimePercent'); }
  };

  const sorted = [...links].sort((a, b) => {
    const av = a[sortKey], bv = b[sortKey];
    if (typeof av === 'number' && typeof bv === 'number') return sortDesc ? bv - av : av - bv;
    return sortDesc ? String(bv).localeCompare(String(av)) : String(av).localeCompare(String(bv));
  });

  const hdr = (label: string, key: SortKey, align = 'text-left') => (
    <th className={`pb-3 font-medium cursor-pointer select-none hover:text-on-surface transition-colors ${align}`} onClick={() => toggle(key)}>
      {label} {sortKey === key ? (sortDesc ? '↓' : '↑') : ''}
    </th>
  );

  return (
    <div className="report-section">
      <div className="bg-surface-container-low rounded-lg p-6 border border-outline-variant/20">
        <div className="flex items-center gap-2 mb-6">
          <span className="material-symbols-outlined text-primary text-xl">router</span>
          <h3 className="text-sm font-headline font-semibold uppercase tracking-wide">ISP Link Performance</h3>
          <span className="ml-auto text-[10px] text-on-surface-variant uppercase tracking-wide">{links.length} links</span>
        </div>
        {links.length === 0 ? (
          <p className="text-xs text-on-surface-variant text-center py-16">No ISP link data for selected range</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-[10px] uppercase tracking-wide text-on-surface-variant border-b border-outline-variant/20">
                  {hdr('Link', 'name')}
                  {hdr('Provider', 'provider')}
                  {hdr('Uptime', 'uptimePercent', 'text-center')}
                  {hdr('Latency', 'avgLatency', 'text-right')}
                  {hdr('Packet Loss', 'avgPacketLoss', 'text-right')}
                  {hdr('Download', 'avgDownload', 'text-right')}
                  {hdr('Upload', 'avgUpload', 'text-right')}
                </tr>
              </thead>
              <tbody className="text-sm">
                {sorted.map((l) => (
                  <tr key={l.id} className="border-b border-outline-variant/10 hover:bg-surface-container-lowest/50 transition-colors">
                    <td className="py-3">
                      <span className="font-headline font-semibold">{l.name}</span>
                    </td>
                    <td className="py-3 text-on-surface-variant">{l.provider}</td>
                    <td className="py-3 text-center">
                      <span className={`inline-flex px-2.5 py-1 rounded-full border text-[11px] font-semibold ${badge(l.uptimePercent)}`}>
                        {l.uptimePercent.toFixed(1)}%
                      </span>
                    </td>
                    <td className="py-3 text-right font-data">{l.avgLatency.toFixed(1)}ms</td>
                    <td className="py-3 text-right font-data">
                      <span className={l.avgPacketLoss > 1 ? 'text-error' : 'text-on-surface'}>
                        {l.avgPacketLoss.toFixed(2)}%
                      </span>
                    </td>
                    <td className="py-3 text-right font-data">{l.avgDownload.toFixed(1)} Mbps</td>
                    <td className="py-3 text-right font-data">{l.avgUpload.toFixed(1)} Mbps</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
