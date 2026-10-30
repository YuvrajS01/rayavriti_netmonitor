import type { DeviceBreakdown } from '../../api/types';
import { useState } from 'react';
import { iconForProtocol } from '../../utils/icons';
import { StatusPill } from '../ui/Status';
import SortHeader from './SortHeader';
import { availabilityStatus } from './reportStatus';

type SortKey = 'deviceName' | 'availabilityPercent' | 'avgResponse' | 'sampleCount' | 'downCount';

function availabilityOf(d: DeviceBreakdown): number {
  if (d.availabilityPercent != null) return d.availabilityPercent;
  return d.sampleCount > 0 ? 100 - (d.downCount / d.sampleCount) * 100 : 100;
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

  const header = { activeKey: sortKey, desc: sortDesc, onSort: toggle };

  return (
    <div className="report-section">
      <section aria-labelledby="device-performance-heading" className="bg-surface-1 rounded-lg p-5 border border-line">
        <div className="flex items-center gap-2 mb-5">
          <h3 id="device-performance-heading" className="text-sm font-semibold text-fg">Per-device performance</h3>
          <span className="ml-auto text-xs text-fg-muted">{devices.length} devices</span>
        </div>
        {devices.length === 0 ? (
          <p className="text-xs text-fg-muted text-center py-16">No device data for selected range</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="text-xs text-fg-muted border-b border-line">
                  <SortHeader label="Device" sortKey="deviceName" {...header} />
                  <SortHeader label="Availability" sortKey="availabilityPercent" align="center" {...header} />
                  <SortHeader label="Avg response" sortKey="avgResponse" align="right" {...header} />
                  <SortHeader label="Samples" sortKey="sampleCount" align="right" {...header} />
                  <SortHeader label="Down" sortKey="downCount" align="right" {...header} />
                </tr>
              </thead>
              <tbody className="text-sm">
                {sorted.map((d) => {
                  const availability = availabilityOf(d);
                  return (
                    <tr key={d.deviceId} className="border-b border-line last:border-b-0 hover:bg-surface-2 transition-colors">
                      <td className="py-2.5 pr-3">
                        <button type="button" onClick={() => onSelectDevice(d.deviceId)} className="flex items-center gap-2 rounded-sm text-left hover:underline">
                          <span className="material-symbols-outlined text-[16px] text-fg-subtle" aria-hidden="true">{iconForProtocol(d.protocol)}</span>
                          <span className="font-semibold text-fg">{d.deviceName}</span>
                          <span className="text-xs text-fg-muted uppercase">{d.protocol}</span>
                        </button>
                      </td>
                      <td className="py-2.5 text-center">
                        <StatusPill status={availabilityStatus(availability)}>
                          <span className="font-mono">{availability.toFixed(1)}%</span>
                        </StatusPill>
                      </td>
                      <td className="py-2.5 text-right font-mono">{d.avgResponse} ms</td>
                      <td className="py-2.5 text-right font-mono text-fg-muted">{d.sampleCount}</td>
                      <td className="py-2.5 text-right font-mono">{d.downCount || '—'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
