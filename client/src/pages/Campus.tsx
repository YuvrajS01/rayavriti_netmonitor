import { useEffect, useState, useMemo, useCallback } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import LocationTree from '../components/LocationTree';
import { listPhase2, type Phase2Row } from '../api/phase2';
import { v1 } from '../api/http';

interface DeviceRow {
  id: number;
  name: string;
  ipAddress: string;
  status: string;
  protocol: string;
  deviceCategory: string;
  locationId: number | null;
}

const statusColors: Record<string, string> = {
  up: 'bg-success',
  down: 'bg-error',
  warning: 'bg-warning',
  degraded: 'bg-warning',
  maintenance: 'bg-info',
  unknown: 'bg-outline',
};

export default function Campus() {
  const [locations, setLocations] = useState<Phase2Row[]>([]);
  const [devices, setDevices] = useState<DeviceRow[]>([]);
  const [selected, setSelected] = useState<Phase2Row | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const [locRes, devRes] = await Promise.all([
          listPhase2('/locations'),
          v1.get('/devices').then((r) => r.data),
        ]);
        setLocations(locRes.data || []);
        const devData = devRes?.data ?? devRes ?? [];
        setDevices(Array.isArray(devData) ? devData : []);
      } catch {
        setLocations([]);
        setDevices([]);
      }
      setLoading(false);
    })();
  }, []);

  // Enrich locations with device counts and status from the device list.
  const enriched = useMemo(() => {
    const statusByLoc: Record<number, Record<string, number>> = {};
    for (const d of devices) {
      if (d.locationId == null) continue;
      if (!statusByLoc[d.locationId]) {
        statusByLoc[d.locationId] = { up: 0, down: 0, warning: 0, maintenance: 0, unknown: 0 };
      }
      const key = d.status in statusByLoc[d.locationId] ? d.status : 'unknown';
      statusByLoc[d.locationId][key]++;
    }
    return locations.map((loc) => {
      const locId = Number(loc.id);
      const st = statusByLoc[locId] || { up: 0, down: 0, warning: 0, maintenance: 0, unknown: 0 };
      const total = Object.values(st).reduce((a, b) => a + b, 0);
      return { ...loc, status: st, device_count: total };
    });
  }, [locations, devices]);

  const selectedDevices = useMemo(() => {
    if (!selected) return [];
    const locId = Number(selected.id);
    return devices.filter((d) => d.locationId === locId);
  }, [selected, devices]);

  // Aggregate stats.
  const stats = useMemo(() => {
    const buildings = locations.filter((l) => l.type === 'building').length;
    const totalDown = devices.filter((d) => d.status === 'down').length;
    return { total: locations.length, buildings, devices: devices.length, down: totalDown };
  }, [locations, devices]);

  const handleSelect = useCallback((loc: Phase2Row) => setSelected(loc), []);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Campus Topology"
        subtitle="Explore your campus network by location — buildings, floors, rooms, and the devices within."
      />

      {/* Stats Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {[
          { label: 'Locations', value: stats.total, icon: 'apartment', color: 'text-primary' },
          { label: 'Buildings', value: stats.buildings, icon: 'domain', color: 'text-info' },
          { label: 'Total Devices', value: stats.devices, icon: 'devices', color: 'text-success' },
          { label: 'Down', value: stats.down, icon: 'error', color: 'text-error' },
        ].map((s) => (
          <Card key={s.label} variant="low" className="p-4">
            <div className="flex items-center justify-between">
              <span className="text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">
                {s.label}
              </span>
              <span className={`material-symbols-outlined text-base ${s.color}`}>{s.icon}</span>
            </div>
            <div className="font-headline text-2xl font-bold mt-1">{s.value}</div>
          </Card>
        ))}
      </div>

      {locations.length === 0 ? (
        <Card variant="low" className="p-8">
          <EmptyState
            icon="account_tree"
            title="No locations configured"
            description="Create your first location in the Location Manager to get started."
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
          {/* Left Panel — Tree */}
          <Card variant="low" className="lg:col-span-4 xl:col-span-3 p-4 max-h-[75vh] overflow-hidden flex flex-col">
            <h2 className="font-headline font-bold text-sm uppercase tracking-wide text-on-surface-variant mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-base text-primary">account_tree</span>
              Location Tree
            </h2>
            <LocationTree
              locations={enriched}
              onSelect={handleSelect}
              selectedId={selected ? Number(selected.id) : null}
              showStatus
              showDeviceCount
            />
          </Card>

          {/* Right Panel — Detail */}
          <Card variant="low" className="lg:col-span-8 xl:col-span-9 p-0 overflow-hidden">
            {selected ? (
              <>
                {/* Location header */}
                <div className="p-5 border-b border-outline-variant/20">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="font-headline font-bold text-xl">{String(selected.name)}</h2>
                      <p className="text-xs text-on-surface-variant uppercase tracking-wide mt-1">
                        {String(selected.type)} {selected.code ? `· ${selected.code}` : ''}
                      </p>
                      {selected.description ? (
                        <p className="text-sm text-on-surface-variant mt-2">{String(selected.description)}</p>
                      ) : null}
                    </div>
                    {/* Status badges */}
                    <div className="flex items-center gap-2 shrink-0">
                      {(() => {
                        const st = (selected as any).status as Record<string, number> | undefined;
                        if (!st) return null;
                        return (
                          <>
                            {st.up > 0 && (
                              <span className="flex items-center gap-1 bg-success/10 text-success px-2 py-1 rounded-md text-xs font-bold">
                                <span className="w-2 h-2 rounded-full bg-success" />
                                {st.up} Up
                              </span>
                            )}
                            {st.down > 0 && (
                              <span className="flex items-center gap-1 bg-error/10 text-error px-2 py-1 rounded-md text-xs font-bold">
                                <span className="w-2 h-2 rounded-full bg-error" />
                                {st.down} Down
                              </span>
                            )}
                            {st.warning > 0 && (
                              <span className="flex items-center gap-1 bg-warning/10 text-warning px-2 py-1 rounded-md text-xs font-bold">
                                <span className="w-2 h-2 rounded-full bg-warning" />
                                {st.warning} Warn
                              </span>
                            )}
                          </>
                        );
                      })()}
                    </div>
                  </div>
                </div>

                {/* Device table */}
                <div className="p-5">
                  <h3 className="font-headline font-bold text-sm uppercase tracking-wide text-on-surface-variant mb-3">
                    Devices at this location ({selectedDevices.length})
                  </h3>
                  {selectedDevices.length === 0 ? (
                    <div className="py-8 text-center text-on-surface-variant text-sm">
                      <span className="material-symbols-outlined text-2xl mb-2 block">devices</span>
                      No devices assigned to this location
                    </div>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b border-outline-variant/20 text-left">
                            <th className="pb-2 text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">Status</th>
                            <th className="pb-2 text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">Name</th>
                            <th className="pb-2 text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">IP Address</th>
                            <th className="pb-2 text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">Protocol</th>
                            <th className="pb-2 text-[10px] uppercase tracking-wide text-on-surface-variant font-bold">Category</th>
                          </tr>
                        </thead>
                        <tbody>
                          {selectedDevices.map((d) => (
                            <tr
                              key={d.id}
                              className="border-b border-outline-variant/10 hover:bg-surface-container-high/60 transition-colors"
                            >
                              <td className="py-2.5 pr-3">
                                <span
                                  className={`w-2.5 h-2.5 rounded-full inline-block ${statusColors[d.status] || 'bg-outline'}`}
                                  title={d.status}
                                />
                              </td>
                              <td className="py-2.5 pr-3 font-medium">{d.name}</td>
                              <td className="py-2.5 pr-3 text-on-surface-variant font-mono text-xs">
                                {d.ipAddress}
                              </td>
                              <td className="py-2.5 pr-3 text-on-surface-variant uppercase text-xs">
                                {d.protocol}
                              </td>
                              <td className="py-2.5 text-on-surface-variant text-xs">
                                {d.deviceCategory || '—'}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              </>
            ) : (
              <div className="flex flex-col items-center justify-center min-h-[400px] text-on-surface-variant">
                <span className="material-symbols-outlined text-5xl mb-3 text-outline">touch_app</span>
                <p className="font-headline font-bold">Select a location</p>
                <p className="text-xs mt-1">Choose a location from the tree to see its devices and status.</p>
              </div>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}
