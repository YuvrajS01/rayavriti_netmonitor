import type { ISPReportLink } from '../../api/reports';
import { useState } from 'react';
import { STATUS_META } from '../../utils/status';
import { StatusPill, StatusShapeIcon } from '../ui/Status';
import SortHeader from './SortHeader';
import { availabilityStatus } from './reportStatus';

type SortKey = 'name' | 'provider' | 'uptimePercent' | 'avgLatency' | 'avgPacketLoss' | 'avgDownload' | 'avgUpload';

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

  const header = { activeKey: sortKey, desc: sortDesc, onSort: toggle };
  const warn = STATUS_META.warning;

  return (
    <div className="report-section">
      <section aria-labelledby="isp-performance-heading" className="bg-surface-1 rounded-lg p-5 border border-line">
        <div className="flex items-center gap-2 mb-5">
          <h3 id="isp-performance-heading" className="text-sm font-semibold text-fg">ISP link performance</h3>
          <span className="ml-auto text-xs text-fg-muted">{links.length} links</span>
        </div>
        {links.length === 0 ? (
          <p className="text-xs text-fg-muted text-center py-16">No ISP link data for selected range</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-line">
                  <SortHeader label="Link" sortKey="name" {...header} />
                  <SortHeader label="Provider" sortKey="provider" {...header} />
                  <SortHeader label="Uptime" sortKey="uptimePercent" align="center" {...header} />
                  <SortHeader label="Latency" sortKey="avgLatency" align="right" {...header} />
                  <SortHeader label="Packet loss" sortKey="avgPacketLoss" align="right" {...header} />
                  <SortHeader label="Download" sortKey="avgDownload" align="right" {...header} />
                  <SortHeader label="Upload" sortKey="avgUpload" align="right" {...header} />
                </tr>
              </thead>
              <tbody className="text-sm">
                {sorted.map((l) => (
                  <tr key={l.id} className="border-b border-line last:border-b-0 hover:bg-surface-2 transition-colors">
                    <td className="py-2.5 pr-3 font-semibold text-fg">{l.name}</td>
                    <td className="py-2.5 pr-3 text-fg-muted">{l.provider}</td>
                    <td className="py-2.5 text-center">
                      <StatusPill status={availabilityStatus(l.uptimePercent)}>
                        <span className="font-mono">{l.uptimePercent.toFixed(1)}%</span>
                      </StatusPill>
                    </td>
                    <td className="py-2.5 text-right font-mono">{l.avgLatency.toFixed(1)} ms</td>
                    <td className="py-2.5 text-right font-mono">
                      {l.avgPacketLoss > 1 ? (
                        <span className={`inline-flex items-center gap-1 ${warn.text}`}>
                          <StatusShapeIcon shape={warn.shape} size={8} label="Above 1%" />
                          {l.avgPacketLoss.toFixed(2)}%
                        </span>
                      ) : (
                        <span className="text-fg">{l.avgPacketLoss.toFixed(2)}%</span>
                      )}
                    </td>
                    <td className="py-2.5 text-right font-mono">{l.avgDownload.toFixed(1)} Mbps</td>
                    <td className="py-2.5 text-right font-mono">{l.avgUpload.toFixed(1)} Mbps</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
