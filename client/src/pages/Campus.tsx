import { useEffect, useState, useMemo, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import EmptyState from '../components/ui/EmptyState';
import LocationTree from '../components/LocationTree';
import type { ResourceRow } from '../api/resources';
import { getDevices } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useResourceList } from '../hooks/useResources';
import type { Device } from '../api/types';
import { useToast } from '../components/ui/useToast';
import CampusMap from '../components/CampusMap';
import FloorPlanView from '../components/FloorPlanView';
import { asText } from '../utils/formatters';
import StatCard from '../components/ui/StatCard';
import LoadingState from '../components/ui/LoadingState';
import { StatusPill } from '../components/ui/Status';
import { TabPanel, Tabs } from '../components/ui/Tabs';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { normalizeStatus, statusMeta } from '../utils/status';

type StatusCounts = { up: number; down: number; warning: number; maintenance: number; unknown: number };

const VIEWS = ['tree', 'map', 'floor'] as const;
const VIEW_TABS = [
  { id: 'tree', label: 'Tree view', icon: 'account_tree' },
  { id: 'map', label: 'Campus map', icon: 'map' },
  { id: 'floor', label: 'Floor plan', icon: 'grid_view' },
] as const;

const EMPTY_LOCATIONS: ResourceRow[] = [];
const EMPTY_DEVICES: Device[] = [];

export default function Campus() {
  const { addToast } = useToast();
  const [selected, setSelected] = useState<ResourceRow | null>(null);
  const [view, setView] = useTabParam(VIEWS, 'view');
  const tabsId = useTabsId();

  const locationsQuery = useResourceList('/locations');
  // Device status is kept live by useLiveQueryBridge.
  const devicesQuery = useQuery({
    queryKey: queryKeys.devices.list(),
    queryFn: ({ signal }) => getDevices({ signal }),
  });
  const locations = locationsQuery.data ?? EMPTY_LOCATIONS;
  const devices = devicesQuery.data?.data ?? EMPTY_DEVICES;
  const loading = locationsQuery.isPending || devicesQuery.isPending;

  const failedAt = Math.max(locationsQuery.errorUpdatedAt, devicesQuery.errorUpdatedAt);
  useEffect(() => {
    if (failedAt) addToast('Failed to load campus data', 'error');
  }, [failedAt, addToast]);

  // Enrich locations with device counts and status aggregated recursively from
  // the whole subtree (rack → floor → building → campus), matching the backend
  // GetTreeWithStatus semantic.
  const enriched = useMemo(() => {
    const empty: StatusCounts = { up: 0, down: 0, warning: 0, maintenance: 0, unknown: 0 };
    const idToLoc = new Map<number, ResourceRow>();
    for (const loc of locations) idToLoc.set(Number(loc.id), loc);

    // Direct (own) counts per location.
    const own: Record<number, StatusCounts> = {};
    for (const loc of locations) own[Number(loc.id)] = { ...empty };
    for (const d of devices) {
      if (d.locationId == null) continue;
      const counts = own[d.locationId];
      if (!counts) continue;
      const s = normalizeStatus(d.status);
      const key: keyof StatusCounts = s in counts ? (s as keyof StatusCounts) : 'unknown';
      counts[key]++;
    }

    // Child index for upward aggregation.
    const kids = new Map<number, number[]>();
    for (const loc of locations) {
      const pid = loc.parent_id != null ? Number(loc.parent_id) : null;
      if (pid != null && idToLoc.has(pid)) {
        const lst = kids.get(pid);
        if (lst) lst.push(Number(loc.id));
        else kids.set(pid, [Number(loc.id)]);
      }
    }

    // Post-order: each node's totals include all descendants.
    const subtotal = new Map<number, StatusCounts>();
    const aggregate = (locId: number): StatusCounts => {
      const st = { ...(own[locId] || empty) };
      for (const cid of kids.get(locId) || []) {
        const cs = aggregate(cid);
        for (const k of Object.keys(st) as (keyof typeof st)[]) st[k] += cs[k];
      }
      subtotal.set(locId, st);
      return st;
    };
    for (const loc of locations) {
      const pid = loc.parent_id != null ? Number(loc.parent_id) : null;
      if (pid == null || !idToLoc.has(pid)) aggregate(Number(loc.id));
    }

    return locations.map((loc) => {
      const locId = Number(loc.id);
      const st = subtotal.get(locId) || own[locId] || { ...empty };
      const total = Object.values(st).reduce((a, b) => a + b, 0);
      return { ...loc, status: st, device_count: total };
    });
  }, [locations, devices]);

  // Device IDs at or under each location (inclusive), for the detail table so a
  // floor/campus selection lists devices that live in descendant racks/rooms.
  const descendantIds = useMemo(() => {
    const idToLoc = new Map<number, ResourceRow>();
    for (const loc of locations) idToLoc.set(Number(loc.id), loc);
    const kids = new Map<number, number[]>();
    for (const loc of locations) {
      const pid = loc.parent_id != null ? Number(loc.parent_id) : null;
      if (pid != null && idToLoc.has(pid)) {
        const lst = kids.get(pid);
        if (lst) lst.push(Number(loc.id));
        else kids.set(pid, [Number(loc.id)]);
      }
    }
    const out = new Map<number, Set<number>>();
    const collect = (locId: number): Set<number> => {
      const set = new Set<number>([locId]);
      for (const cid of kids.get(locId) || []) {
        for (const id of collect(cid)) set.add(id);
      }
      out.set(locId, set);
      return set;
    };
    for (const loc of locations) {
      const pid = loc.parent_id != null ? Number(loc.parent_id) : null;
      if (pid == null || !idToLoc.has(pid)) collect(Number(loc.id));
    }
    return out;
  }, [locations]);

  const selectedDevices = useMemo(() => {
    if (!selected) return [];
    const locId = Number(selected.id);
    const scope = descendantIds.get(locId);
    if (!scope) return [];
    return devices.filter((d) => d.locationId != null && scope.has(d.locationId));
  }, [selected, devices, descendantIds]);

  // Aggregate stats.
  const stats = useMemo(() => {
    const buildings = locations.filter((l) => l.type === 'building').length;
    const totalDown = devices.filter((d) => normalizeStatus(d.status) === 'down').length;
    return { total: locations.length, buildings, devices: devices.length, down: totalDown };
  }, [locations, devices]);

  const handleSelect = useCallback((loc: ResourceRow) => setSelected(loc), []);

  if (loading) {
    return (
      <LoadingState message="Loading campus…" />
    );
  }

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Campus topology"
        subtitle="Explore your campus network by location — buildings, floors, rooms, and the devices within."
      />

      {/* Stats Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Locations" value={stats.total} icon="apartment" />
        <StatCard label="Buildings" value={stats.buildings} icon="domain" />
        <StatCard label="Total devices" value={stats.devices} icon="devices" />
        <StatCard label="Down" value={stats.down} icon={statusMeta('down').icon} color={stats.down ? statusMeta('down').text : 'text-fg'} />
      </div>

      <Tabs label="Campus views" idBase={tabsId} items={VIEW_TABS} value={view} onChange={setView} />

      <TabPanel idBase={tabsId} id={view} value={view}>
      {locations.length === 0 ? (
        <Card variant="low" className="p-8">
          <EmptyState
            icon="account_tree"
            title="No locations configured"
            description="Create your first location in the Location Manager to get started."
          />
        </Card>
      ) : view === 'map' ? (
        <CampusMap locations={enriched} selectedId={selected ? Number(selected.id) : null} onSelect={handleSelect} />
      ) : view === 'floor' ? (
        <FloorPlanView location={selected} locations={locations} devices={devices} />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
          {/* Left Panel — Tree */}
          <Card variant="low" className="lg:col-span-4 xl:col-span-3 p-4 max-h-[75vh] overflow-hidden flex flex-col">
            <h2 className="font-semibold text-sm text-fg mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-base text-fg-muted" aria-hidden="true">account_tree</span>
              Location tree
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
                <div className="p-5 border-b border-line">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="font-semibold text-xl">{String(selected.name)}</h2>
                      <p className="text-xs text-fg-muted mt-1 capitalize">
                        {asText(selected.type)} {selected.code ? `· ${asText(selected.code)}` : ''}
                      </p>
                      {selected.description ? (
                        <p className="text-sm text-fg-muted mt-2">{asText(selected.description)}</p>
                      ) : null}
                    </div>
                    {/* Status badges */}
                    <div className="flex items-center gap-2 shrink-0">
                      {(() => {
                        const st = (selected as ResourceRow & { status?: StatusCounts }).status;
                        if (!st) return null;
                        return (
                          <>
                            {st.down > 0 && <StatusPill status="down">{st.down} down</StatusPill>}
                            {st.warning > 0 && <StatusPill status="warning">{st.warning} warning</StatusPill>}
                            {st.up > 0 && <StatusPill status="up">{st.up} up</StatusPill>}
                          </>
                        );
                      })()}
                    </div>
                  </div>
                </div>

                {/* Device table */}
                <div className="p-5">
                  <h3 className="font-semibold text-sm text-fg mb-3">
                    Devices at this location ({selectedDevices.length})
                  </h3>
                  {selectedDevices.length === 0 ? (
                    <div className="py-8 text-center text-fg-muted text-sm">
                      <span className="material-symbols-outlined text-2xl mb-2 block text-fg-subtle" aria-hidden="true">devices</span>
                      No devices assigned to this location
                    </div>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b border-line text-left">
                            <th className="pb-2 text-xs text-fg-muted font-semibold">Status</th>
                            <th className="pb-2 text-xs text-fg-muted font-semibold">Name</th>
                            <th className="pb-2 text-xs text-fg-muted font-semibold">IP address</th>
                            <th className="pb-2 text-xs text-fg-muted font-semibold">Protocol</th>
                            <th className="pb-2 text-xs text-fg-muted font-semibold">Category</th>
                          </tr>
                        </thead>
                        <tbody>
                          {selectedDevices.map((d) => (
                            <tr
                              key={d.id}
                              className="border-b border-line hover:bg-surface-2 transition-colors"
                            >
                              <td className="py-2.5 pr-3">
                                <StatusPill status={d.status} />
                              </td>
                              <td className="py-2.5 pr-3 font-medium">{d.name}</td>
                              <td className="py-2.5 pr-3 text-fg-muted font-mono text-xs">
                                {d.ipAddress}
                              </td>
                              <td className="py-2.5 pr-3 text-fg-muted text-xs uppercase">
                                {d.protocol}
                              </td>
                              <td className="py-2.5 text-fg-muted text-xs capitalize">
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
              <div className="flex flex-col items-center justify-center min-h-[400px] text-fg-muted">
                <span className="material-symbols-outlined text-5xl mb-3 text-fg-subtle" aria-hidden="true">touch_app</span>
                <p className="font-semibold text-fg">Select a location</p>
                <p className="text-sm mt-1">Choose a location from the tree to see its devices and status.</p>
              </div>
            )}
          </Card>
        </div>
      )}
      </TabPanel>
    </div>
  );
}
