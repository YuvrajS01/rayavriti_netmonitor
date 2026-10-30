import { useMemo } from 'react';
import type { ResourceRow } from '../api/resources';
import type { Device } from '../api/types';
import Card from './ui/Card';

interface FloorPlanViewProps {
  location: ResourceRow | null;
  locations: ResourceRow[];
  devices: Device[];
  onDeviceClick?: (device: Device) => void;
}

const UNKNOWN_STATUS_COLOR = 'bg-line-strong border-line-strong';
const statusColors: Record<string, string> = {
  up: 'bg-success border-success',
  down: 'bg-error border-error',
  warning: 'bg-warning border-warning',
  degraded: 'bg-warning border-warning',
  maintenance: 'bg-info border-info',
  unknown: UNKNOWN_STATUS_COLOR,
};

const categoryIcons: Record<string, string> = {
  router: 'router',
  switch: 'hub',
  server: 'dns',
  firewall: 'security',
  ap: 'wifi',
};

function getCategoryIcon(category?: string) {
  if (!category) return 'devices';
  return categoryIcons[category.toLowerCase()] || 'devices';
}

function parseRackPosition(pos?: string): number | null {
  if (!pos) return null;
  const match = pos.match(/\d+/);
  if (match) return parseInt(match[0], 10);
  return null;
}

export default function FloorPlanView({
  location,
  locations,
  devices,
  onDeviceClick,
}: FloorPlanViewProps) {
  if (!location) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[440px] bg-inset rounded-lg border border-line p-8 text-center">
        <span className="material-symbols-outlined text-5xl text-fg-subtle mb-4">architecture</span>
        <h2 className="font-sans text-xl font-semibold">Floor plan workspace</h2>
        <p className="text-sm text-fg-muted mt-2 max-w-md">
          Select a location from the tree to view its floor plan
        </p>
      </div>
    );
  }

  // Determine which locations to display
  const locId = Number(location.id);
  const children = locations.filter((l) => Number(l.parent_id) === locId);
  const locType = String(location.type).toLowerCase();
  const isTerminal = children.length === 0 && (locType === 'room' || locType === 'rack');
  const items = isTerminal ? [location] : children;

  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[440px] bg-inset rounded-lg border border-line p-8 text-center">
        <span className="material-symbols-outlined text-5xl text-fg-subtle mb-4">architecture</span>
        <h2 className="font-sans text-xl font-semibold">No floor plan data</h2>
        <p className="text-sm text-fg-muted mt-2">
          This location has no rooms or racks configured.
        </p>
      </div>
    );
  }

  return (
    <div className="min-h-[440px] bg-inset rounded-lg border border-line p-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {items.map((item) => {
          const itemId = Number(item.id);
          const locDevices = devices.filter((d) => d.locationId === itemId);
          
          if (item.type === 'rack') {
            return (
              <RackVisual
                key={String(item.id)}
                location={item}
                devices={locDevices}
                onClick={onDeviceClick}
              />
            );
          }

          return (
            <RoomVisual
              key={String(item.id)}
              location={item}
              devices={locDevices}
              onClick={onDeviceClick}
            />
          );
        })}
      </div>
    </div>
  );
}

function RoomVisual({
  location,
  devices,
  onClick,
}: {
  location: ResourceRow;
  devices: Device[];
  onClick?: (d: Device) => void;
}) {
  return (
    <Card variant="low" className="flex flex-col h-full overflow-hidden">
      <div className="p-4 border-b border-line flex items-center gap-2">
        <span className="material-symbols-outlined text-primary">meeting_room</span>
        <div>
          <h3 className="font-sans font-semibold text-base">{String(location.name)}</h3>
          <p className="text-[10px] uppercase tracking-wide text-fg-muted">Room</p>
        </div>
      </div>
      <div className="p-4 flex-1 flex flex-col gap-3">
        {devices.length === 0 ? (
          <div className="text-center text-fg-muted text-sm py-4">
            No devices
          </div>
        ) : (
          devices.map((d) => {
            const sc = statusColors[d.status] || UNKNOWN_STATUS_COLOR;
            const bgClass = sc.split(' ')[0];
            const borderClass = sc.split(' ')[1];
            
            return (
              <div
                key={d.id}
                onClick={() => onClick?.(d)}
                className={`flex items-center gap-3 p-3 rounded bg-surface-2 border-l-4 ${borderClass} border-y border-r border-y-line border-r-line hover:bg-surface-2 transition-colors cursor-pointer`}
              >
                <div className="relative shrink-0">
                  <span className={`absolute -inset-1 rounded-full ${bgClass} opacity-20 status-dot-live`}></span>
                  <span className={`relative block w-2.5 h-2.5 rounded-full ${bgClass}`}></span>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="material-symbols-outlined text-[16px] text-fg-muted">
                      {getCategoryIcon(d.deviceCategory)}
                    </span>
                    <span className="font-semibold text-sm truncate">{d.name}</span>
                  </div>
                  <div className="flex items-center justify-between mt-1">
                    <span className="font-mono text-[11px] text-fg-muted">{d.ipAddress}</span>
                    <span className="text-[9px] uppercase tracking-wide bg-surface-3 px-1.5 py-0.5 rounded text-fg-muted">
                      {d.protocol}
                    </span>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>
    </Card>
  );
}

function RackVisual({
  location,
  devices,
  onClick,
}: {
  location: ResourceRow;
  devices: Device[];
  onClick?: (d: Device) => void;
}) {
  const RACK_UNITS = 42;
  const units = Array.from({ length: RACK_UNITS }, (_, i) => RACK_UNITS - i);

  // Place devices that have a rack position; auto-assign the rest to free
  // bottom-up slots so a rack with devices never renders as an empty diagram.
  const devicesByUnit = useMemo(() => {
    const map = new Map<number, Device>();
    const unplaced: Device[] = [];
    devices.forEach((d) => {
      const u = parseRackPosition(d.rackPosition);
      if (u && u >= 1 && u <= RACK_UNITS) {
        map.set(u, d);
      } else {
        unplaced.push(d);
      }
    });
    if (unplaced.length > 0) {
      let next = 1;
      for (const d of unplaced) {
        while (next <= RACK_UNITS && map.has(next)) next += 1;
        if (next > RACK_UNITS) break;
        map.set(next, d);
        next += 1;
      }
    }
    return map;
  }, [devices]);

  return (
    <Card variant="low" className="flex flex-col h-[500px] overflow-hidden">
      <div className="p-4 border-b border-line flex items-center gap-2 shrink-0">
        <span className="material-symbols-outlined text-primary">dns</span>
        <div>
          <h3 className="font-sans font-semibold text-base">{String(location.name)}</h3>
          <p className="text-[10px] uppercase tracking-wide text-fg-muted">42U Rack</p>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4 bg-inset">
        <div className="flex flex-col gap-[2px] border-x-4 border-surface-3 rounded-sm bg-surface-3">
          {units.map((u) => {
            const device = devicesByUnit.get(u);
            
            if (device) {
              const sc = statusColors[device.status] || UNKNOWN_STATUS_COLOR;
              const borderClass = sc.split(' ')[1];
              
              return (
                <div
                  key={u}
                  onClick={() => onClick?.(device)}
                  className={`flex items-center h-8 bg-surface-2 border-l-4 ${borderClass} px-2 hover:bg-surface-2 cursor-pointer transition-colors group`}
                  title={`${device.name} (${device.ipAddress})`}
                >
                  <div className="w-5 text-[9px] text-fg-muted font-mono text-right pr-2 select-none">
                    {u}
                  </div>
                  <div className="flex-1 min-w-0 flex items-center gap-2">
                    <span className="material-symbols-outlined text-[14px] text-fg-muted">
                      {getCategoryIcon(device.deviceCategory)}
                    </span>
                    <span className="text-xs font-medium truncate group-hover:text-primary transition-colors">
                      {device.name}
                    </span>
                  </div>
                </div>
              );
            }
            
            return (
              <div key={u} className="flex items-center h-8 bg-inset border border-dashed border-line px-2 opacity-50">
                <div className="w-5 text-[9px] text-line font-mono text-right pr-2 select-none">
                  {u}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </Card>
  );
}
