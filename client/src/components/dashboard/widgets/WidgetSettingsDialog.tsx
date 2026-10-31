import { useId, useMemo, useState, type FormEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getDevices } from '../../../api/devices';
import { HISTORY_MAX_DEVICES, HISTORY_RANGES } from '../../../api/history';
import { getDeviceInterfaces } from '../../../api/interfaces';
import { queryKeys } from '../../../api/queryKeys';
import type { DashboardWidget } from '../../../api/types';
import { useResourceList } from '../../../hooks/useResources';
import Button from '../../ui/Button';
import Dialog from '../../ui/Dialog';
import { widgetMeta } from './catalog';
import { settingsProblem, str } from './widgetData';

type Config = Record<string, unknown>;

interface Props {
  widget: DashboardWidget;
  onClose: () => void;
  onSave: (config: Config) => void;
}

const inputClass = 'w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary';
const labelClass = 'mb-1.5 block text-xs font-semibold text-fg-muted';

/** Settings for one widget: a title for all, plus the type's own fields. */
export default function WidgetSettingsDialog({ widget, onClose, onSave }: Props) {
  const meta = widgetMeta(widget.type);
  const [config, setConfig] = useState<Config>({ ...(meta?.defaultConfig ?? {}), ...(widget.config ?? {}) });
  const [error, setError] = useState('');
  const titleId = useId();
  const set = (patch: Config) => {
    setError('');
    setConfig((c) => ({ ...c, ...patch }));
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const problem = settingsProblem(widget.type, config);
    if (problem) {
      setError(problem);
      return;
    }
    const title = typeof config.title === 'string' ? config.title.trim() : '';
    const out: Config = { ...config, title: title || undefined };
    onSave(Object.fromEntries(Object.entries(out).filter(([, v]) => v !== undefined && v !== '')));
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={`${meta?.label ?? 'Widget'} settings`}
      closeOnBackdrop={false}
      size="md"
      footer={
        <>
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" form="widget-settings-form">Apply</Button>
        </>
      }
    >
      <form id="widget-settings-form" onSubmit={submit} className="space-y-4" noValidate>
        <div>
          <label htmlFor={titleId} className={labelClass}>Title</label>
          <input
            id={titleId}
            className={inputClass}
            value={typeof config.title === 'string' ? config.title : ''}
            placeholder={meta?.label}
            maxLength={80}
            onChange={(e) => set({ title: e.target.value })}
          />
        </div>
        {widget.type === 'device.history' && <DeviceHistoryFields config={config} set={set} />}
        {widget.type === 'interface.throughput' && <InterfaceFields config={config} set={set} />}
        {widget.type === 'device.list' && <DeviceListFields config={config} set={set} />}
        {error && <p role="alert" className="text-sm text-error">{error}</p>}
      </form>
    </Dialog>
  );
}


interface FieldsProps {
  config: Config;
  set: (patch: Config) => void;
}

function RangeField({ config, set }: FieldsProps) {
  const id = useId();
  return (
    <div>
      <label htmlFor={id} className={labelClass}>Time range</label>
      <select id={id} className={`${inputClass} cursor-pointer`} value={str(config.range, '24h')} onChange={(e) => set({ range: e.target.value })}>
        {HISTORY_RANGES.map((r) => <option key={r.id} value={r.id}>Last {r.label}</option>)}
      </select>
    </div>
  );
}

function useDeviceList() {
  return useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }) });
}

function DeviceHistoryFields({ config, set }: FieldsProps) {
  const devices = useDeviceList();
  const [filter, setFilter] = useState('');
  const filterId = useId();
  const selected = useMemo(() => new Set(Array.isArray(config.deviceIds) ? (config.deviceIds as number[]) : []), [config.deviceIds]);
  const shown = (devices.data?.data ?? []).filter((d) => {
    const q = filter.trim().toLowerCase();
    return !q || d.name.toLowerCase().includes(q) || d.ipAddress.toLowerCase().includes(q);
  });
  const toggle = (id: number) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    set({ deviceIds: [...next] });
  };

  return (
    <>
      <fieldset>
        <legend className={labelClass}>Devices <span className="font-normal">({selected.size} of up to {HISTORY_MAX_DEVICES} chosen)</span></legend>
        <label htmlFor={filterId} className="sr-only">Filter devices</label>
        <input id={filterId} className={`${inputClass} mb-2`} placeholder="Filter by name or address" value={filter} onChange={(e) => setFilter(e.target.value)} />
        <div className="max-h-56 overflow-y-auto rounded-md border border-line">
          {devices.isPending ? (
            <p role="status" className="p-3 text-xs text-fg-muted">Loading devices…</p>
          ) : !shown.length ? (
            <p className="p-3 text-xs text-fg-muted">No devices match.</p>
          ) : (
            shown.map((d) => {
              const checked = selected.has(d.id);
              return (
                <label key={d.id} className="flex cursor-pointer items-center gap-2.5 border-b border-line px-3 py-2 text-sm last:border-0 hover:bg-surface-2">
                  <input type="checkbox" checked={checked} disabled={!checked && selected.size >= HISTORY_MAX_DEVICES} onChange={() => toggle(d.id)} className="accent-[var(--color-primary)]" />
                  <span className="min-w-0 flex-1 truncate">{d.name}</span>
                  <span className="font-mono text-xs text-fg-muted">{d.ipAddress}</span>
                </label>
              );
            })
          )}
        </div>
      </fieldset>
      <RangeField config={config} set={set} />
    </>
  );
}

function InterfaceFields({ config, set }: FieldsProps) {
  const devices = useDeviceList();
  const deviceFieldId = useId();
  const ifFieldId = useId();
  const deviceId = typeof config.deviceId === 'number' ? config.deviceId : undefined;
  const snmp = (devices.data?.data ?? []).filter((d) => d.protocol === 'snmp');
  const ifaces = useQuery({
    queryKey: queryKeys.devices.interfaces(deviceId ?? 0),
    queryFn: ({ signal }) => getDeviceInterfaces(deviceId!, { signal }),
    enabled: deviceId != null,
  });

  return (
    <>
      <div>
        <label htmlFor={deviceFieldId} className={labelClass}>SNMP device</label>
        <select
          id={deviceFieldId}
          className={`${inputClass} cursor-pointer`}
          value={deviceId ?? ''}
          onChange={(e) => {
            const id = e.target.value ? Number(e.target.value) : undefined;
            set({ deviceId: id, deviceName: snmp.find((d) => d.id === id)?.name, ifIndex: undefined, ifName: undefined });
          }}
        >
          <option value="">{devices.isPending ? 'Loading devices…' : snmp.length ? 'Choose a device' : 'No SNMP devices'}</option>
          {snmp.map((d) => <option key={d.id} value={d.id}>{d.name} ({d.ipAddress})</option>)}
        </select>
      </div>
      <div>
        <label htmlFor={ifFieldId} className={labelClass}>Interface</label>
        <select
          id={ifFieldId}
          className={`${inputClass} cursor-pointer`}
          disabled={deviceId == null}
          value={typeof config.ifIndex === 'number' ? config.ifIndex : ''}
          onChange={(e) => {
            const idx = e.target.value ? Number(e.target.value) : undefined;
            set({ ifIndex: idx, ifName: ifaces.data?.find((i) => i.ifIndex === idx)?.name });
          }}
        >
          <option value="">
            {deviceId == null ? 'Choose a device first' : ifaces.isPending ? 'Loading interfaces…' : ifaces.data?.length ? 'Choose an interface' : 'No interface data in the last hour'}
          </option>
          {(ifaces.data ?? []).map((i) => (
            <option key={i.ifIndex} value={i.ifIndex}>{i.name} (ifIndex {i.ifIndex}{i.operStatus === 1 ? '' : ', down'})</option>
          ))}
        </select>
      </div>
      <RangeField config={config} set={set} />
    </>
  );
}

function DeviceListFields({ config, set }: FieldsProps) {
  const locations = useResourceList('/locations');
  const ids = { location: useId(), status: useId(), tag: useId(), limit: useId() };
  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <label htmlFor={ids.location} className={labelClass}>Location</label>
          <select
            id={ids.location}
            className={`${inputClass} cursor-pointer`}
            value={typeof config.locationId === 'number' ? config.locationId : ''}
            onChange={(e) => set({ locationId: e.target.value ? Number(e.target.value) : undefined })}
          >
            <option value="">Any location</option>
            {(locations.data ?? []).map((l) => <option key={String(l.id)} value={Number(l.id)}>{String(l.name)}</option>)}
          </select>
        </div>
        <div>
          <label htmlFor={ids.status} className={labelClass}>Show</label>
          <select id={ids.status} className={`${inputClass} cursor-pointer`} value={str(config.status, 'all')} onChange={(e) => set({ status: e.target.value })}>
            <option value="all">All devices</option>
            <option value="problems">Devices not up</option>
            <option value="down">Down devices only</option>
          </select>
        </div>
        <div>
          <label htmlFor={ids.tag} className={labelClass}>Tag</label>
          <input id={ids.tag} className={inputClass} placeholder="Any tag" value={typeof config.tag === 'string' ? config.tag : ''} onChange={(e) => set({ tag: e.target.value })} maxLength={64} />
        </div>
        <div>
          <label htmlFor={ids.limit} className={labelClass}>Devices shown</label>
          <input id={ids.limit} type="number" min={1} max={50} className={inputClass} value={typeof config.limit === 'number' ? config.limit : ''} onChange={(e) => set({ limit: e.target.value === '' ? '' : Number(e.target.value) })} />
        </div>
      </div>
    </>
  );
}
