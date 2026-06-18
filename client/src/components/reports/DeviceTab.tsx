import type { DeviceBreakdown } from '../../api/types';
import { useState } from 'react';

type SortKey = 'deviceName' | 'availabilityPercent' | 'avgResponse' | 'sampleCount' | 'downCount';

function badge(avail: number) {
  if (avail >= 99) return 'bg-primary/15 text-primary border-primary/30';
  if (avail >= 95) return 'bg-warning/15 text-warning border-warning/30';
  return 'bg-error/15 text-error border-error/30';
}

export default function DeviceTab({ devices, onSelectDevice }: { devices: DeviceBreakdown[]; onSelectDevice: (id: number) => void }) {
  const [sortKey, setSortKey] = useState<SortKey>('availabilityPercent');
  const [sortDesc, setSortDesc] = useState(false);

  const toggle = (key: SortKey) => {
    if (sortKey === key) setSortDesc(!sortDesc);
    else { setSortKey(key); setSortDesc(key === 'availabilityPercent'); }
  };

  const sorted = [...devices].sort((a, b) => {
    const av = a[sortKey], bv = b[sortKey];
    if (typeof av === 'number' && typeof bv === 'number') return sortDesc ? bv - av : av - bv;
    return sortDesc ? String(bv).localeCompare(String(av)) : String(av).localeCompare(String(bv));
  });

  const hdr = (label: string, key: SortKey, align = 'text-left') => (
    <th className={`pb-3 font-medium cursor-pointer select-none hover:text-primary transition-colors ${align}`} onClick={() => toggle(key)}>
      {label} {sortKey === key ? (sortDesc ? '↓' : '↑') : ''}
    </th>
  );

  return (
    <div className="report-section">
      <div className="bg-surface-container-high rounded-lg p-6 border border-outline-variant/20">
        <div className="flex items-center gap-2 mb-6">
          <span className="material-symbols-outlined text-primary text-xl">devices</span>
          <h3 className="text-sm font-headline font-bold uppercase tracking-wide">Per-Device Performance</h3>
          <span className="ml-auto text-[10px] text-on-surface-variant uppercase tracking-wide">{devices.length} devices</span>
        </div>
        {devices.length === 0 ? (
          <p className="text-xs text-on-surface-variant text-center py-16">No device data for selected range</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-[10px] uppercase tracking-wide text-on-surface-variant border-b border-outline-variant/20">
                  {hdr('Device', 'deviceName')}
                  {hdr('Availability', 'availabilityPercent', 'text-center')}
                  {hdr('Avg Response', 'avgResponse', 'text-right')}
                  {hdr('Samples', 'sampleCount', 'text-right')}
                  {hdr('Down', 'downCount', 'text-right')}
                </tr>
              </thead>
              <tbody className="text-sm">
                {sorted.map((d) => (
                  <tr key={d.deviceId} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-colors cursor-pointer group" onClick={() => onSelectDevice(d.deviceId)}>
                    <td className="py-3">
                      <div className="flex items-center gap-2">
                        <span className="material-symbols-outlined text-sm opacity-60">
                          {d.protocol === 'ping' ? 'router' : d.protocol === 'http' || d.protocol === 'https' ? 'public' : d.protocol === 'system' ? 'memory' : d.protocol === 'snmp' ? 'settings_input_antenna' : 'hub'}
                        </span>
                        <span className="font-headline font-semibold group-hover:text-primary transition-colors">{d.deviceName}</span>
                        <span className="text-[10px] text-on-surface-variant uppercase">{d.protocol}</span>
                      </div>
                    </td>
                    <td className="py-3 text-center">
                      <span className={`inline-flex px-2.5 py-1 rounded-full border text-[11px] font-bold ${badge(d.availabilityPercent ?? 100 - (d.sampleCount > 0 ? (d.downCount / d.sampleCount) * 100 : 0))}`}>
                        {d.availabilityPercent ?? (d.sampleCount > 0 ? (100 - (d.downCount / d.sampleCount) * 100).toFixed(1) : '100')}%
                      </span>
                    </td>
                    <td className="py-3 text-right font-mono">{d.avgResponse}ms</td>
                    <td className="py-3 text-right font-mono text-on-surface-variant">{d.sampleCount}</td>
                    <td className="py-3 text-right font-mono text-error">{d.downCount || '—'}</td>
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
