import { useState, useEffect, useCallback, useMemo, type FormEvent } from 'react';
import { getDevices, getLatestMetrics, addDevice, deleteDevice } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { Device, Metric } from '../api/types';
import DeviceModal from '../components/DeviceModal';
import ConfirmDialog from '../components/ConfirmDialog';
import StatCard from '../components/ui/StatCard';
import Button from '../components/ui/Button';
import SectionHeader from '../components/ui/SectionHeader';
import { useToast } from '../components/ui/useToast';
import { DevicesSkeleton } from '../components/dashboard/DevicesSkeleton';
import { statusTextColor, statusBgColor } from '../utils/colors';
import { iconForProtocol } from '../utils/icons';

function statusOf(device: Device, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(device.id)?.status || device.status || 'unknown';
}

const STATUS_BORDER_HOVER: Record<string, string> = {
  down: 'hover:border-error',
  warning: 'hover:border-amber-500',
  degraded: 'hover:border-amber-500',
  unknown: 'hover:border-outline-variant',
};

const STATUS_GROUP_HOVER_TEXT: Record<string, string> = {
  down: 'group-hover:text-error',
  warning: 'group-hover:text-amber-500',
  degraded: 'group-hover:text-amber-500',
  unknown: 'group-hover:text-outline',
};

export default function Devices() {
  const { addToast } = useToast();
  const [devices, setDevices] = useState<Device[]>([]);
  const [metricsMap, setMetricsMap] = useState<Map<number, Metric>>(new Map());
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [formProtocol, setFormProtocol] = useState('https');
  const [formPort, setFormPort] = useState(443);
  const [snmpCommunity, setSnmpCommunity] = useState('public');
  const [snmpVersion, setSnmpVersion] = useState('2c');
  const [deleteTarget, setDeleteTarget] = useState<Device | null>(null);

  const load = useCallback(async () => {
    const [dRes, mRes] = await Promise.all([getDevices(), getLatestMetrics()]);
    setDevices(dRes.data || []);
    setMetricsMap(new Map((mRes.data || []).map((m) => [m.deviceId, m])));
    setLoading(false);
  }, []);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { load(); }, [load]);

  useSocket({
    onMetricUpdate: (metric) => {
      const m = metric as unknown as Metric;
      setMetricsMap((prev) => {
        const next = new Map(prev);
        next.set(m.deviceId, m);
        return next;
      });
    },
    onDeviceStatus: (status) => {
      const s = status as { device_id: number; new_status: string };
      setDevices((prev) => prev.map((d) =>
        d.id === s.device_id ? { ...d, status: s.new_status as Device['status'] } : d
      ));
    },
  });

  const filtered = useMemo(() => devices.filter((d) => {
    const status = statusOf(d, metricsMap);
    const text = `${d.name} ${d.ipAddress} ${d.protocol}`.toLowerCase();
    const matchSearch = !search || text.includes(search.toLowerCase());
    let matchStatus = true;
    if (statusFilter === 'up') matchStatus = status === 'up' || status === 'ok';
    else if (statusFilter === 'warning') matchStatus = status === 'warning' || status === 'degraded';
    else if (statusFilter === 'down') matchStatus = status === 'down';
    else if (statusFilter === 'unknown') matchStatus = status === 'unknown';
    return matchSearch && matchStatus;
  }), [devices, metricsMap, search, statusFilter]);

  const handleAdd = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const payload: Parameters<typeof addDevice>[0] = {
      name: fd.get('name') as string,
      ipAddress: fd.get('host') as string,
      protocol: formProtocol,
      port: formPort,
      interval: Number(fd.get('interval') || 60),
    };
    if (formProtocol === 'snmp') {
      payload.snmpCommunity = snmpCommunity.trim() || 'public';
      payload.snmpVersion = snmpVersion;
    }
    const result = await addDevice(payload);
    if (result?.data) {
      setDevices((prev) => [...prev, result.data]);
    }
    e.currentTarget.reset();
    setShowForm(false);
    setFormProtocol('https');
    setFormPort(443);
    setSnmpCommunity('public');
    setSnmpVersion('2c');
    load().catch(() => {});
  };

  const handleDelete = (device: Device) => {
    setDeleteTarget(device);
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    await deleteDevice(deleteTarget.id);
    setDevices((prev) => prev.filter((d) => d.id !== deleteTarget.id));
    addToast(`Deleted "${deleteTarget.name}"`, 'success');
    setDeleteTarget(null);
    load().catch(() => {});
  };

  // Stats
  const total = devices.length;
  const up = useMemo(() => devices.filter((d) => { const s = statusOf(d, metricsMap); return s === 'up' || s === 'ok'; }).length, [devices, metricsMap]);
  const down = useMemo(() => devices.filter((d) => statusOf(d, metricsMap) === 'down').length, [devices, metricsMap]);
  const warn = useMemo(() => devices.filter((d) => { const s = statusOf(d, metricsMap); return s === 'warning' || s === 'degraded'; }).length, [devices, metricsMap]);

  return (
    <div>
      {loading ? (
        <DevicesSkeleton />
      ) : (
      <>
      {/* Header */}
      <SectionHeader
        title="My Devices"
        subtitle="Centralized node management. Monitor real-time telemetry across your infrastructure."
        action={
          <Button onClick={() => setShowForm(!showForm)} icon="add_circle">
            ADD DEVICE
          </Button>
        }
      />

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-12">
        <StatCard label="TOTAL NODES" value={total} />
        <StatCard label="ACTIVE (UP)" value={up} />
        <StatCard label="CRITICAL (DOWN)" value={down} color="text-error" accentColor="#ff4444" />
        <StatCard label="WARNINGS" value={warn} color="text-amber-500" accentColor="#f59e0b" />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4 mb-6">
        <div className="flex-1 min-w-48">
          <input value={search} onChange={(e) => setSearch(e.target.value)} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="Search devices..." />
        </div>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary">
          <option value="all">All statuses</option>
          <option value="up">Up</option>
          <option value="warning">Warning</option>
          <option value="down">Down</option>
          <option value="unknown">Unknown</option>
        </select>
      </div>

      {/* Add Device Form */}
      {showForm && (
        <div className="bg-surface-container-low rounded-xl p-6 border border-outline-variant/20 mb-6">
          <h3 className="text-sm font-headline font-bold mb-4 uppercase tracking-widest">New Device</h3>
          <form onSubmit={handleAdd} className="grid grid-cols-1 md:grid-cols-5 gap-3">
            <input name="name" required placeholder="Name" className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none" />
            <input name="host" required placeholder="Host/IP" className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none" />
            {['https', 'http', 'snmp', 'port'].includes(formProtocol) && (
              <input
                name="port"
                type="number"
                min="1"
                max="65535"
                value={formPort || ''}
                onChange={(e) => setFormPort(Number(e.target.value || 0))}
                placeholder={formProtocol === 'port' ? 'Port to check' : 'Port'}
                required={formProtocol === 'port'}
                className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none"
              />
            )}
            <select
              name="protocol"
              value={formProtocol}
              onChange={(e) => {
                const next = e.target.value;
                setFormProtocol(next);
                if (next === 'https') setFormPort(443);
                if (next === 'http') setFormPort(80);
                if (next === 'snmp') setFormPort(161);
                if (next === 'ping') setFormPort(0);
                if (next === 'port') setFormPort(0);
              }}
              className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none"
            >
              <option value="https">HTTPS</option>
              <option value="http">HTTP</option>
              <option value="ping">Ping (ICMP)</option>
              <option value="port">TCP Port</option>
              <option value="system">System</option>
              <option value="snmp">SNMP</option>
            </select>
            <input name="interval" type="number" min="10" defaultValue={60} placeholder="Interval (s)" className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none" />
            {formProtocol === 'snmp' && (
              <>
                <input
                  name="snmpCommunity"
                  value={snmpCommunity}
                  onChange={(e) => setSnmpCommunity(e.target.value)}
                  placeholder="SNMP Community"
                  className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none md:col-span-2"
                />
                <select
                  name="snmpVersion"
                  value={snmpVersion}
                  onChange={(e) => setSnmpVersion(e.target.value)}
                  className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none"
                >
                  <option value="2c">SNMP v2c</option>
                  <option value="1">SNMP v1</option>
                </select>
              </>
            )}
            <button className="md:col-span-5 bg-primary text-on-primary rounded-lg px-4 py-2 text-xs font-bold tracking-widest uppercase hover:brightness-110 active:scale-95 transition-[filter,transform]">Add Device</button>
          </form>
        </div>
      )}

      {/* Device Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filtered.map((device) => {
          const status = statusOf(device, metricsMap);
          const metric = metricsMap.get(device.id);
          const hoverBorder = STATUS_BORDER_HOVER[status] || 'hover:border-primary/20';
          const hoverText = STATUS_GROUP_HOVER_TEXT[status] || 'group-hover:text-primary';
          return (
            <div 
              key={device.id} 
              role="button"
              tabIndex={0}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSelectedDevice(device); } }}
              className={`bg-surface-container-low rounded-xl group overflow-hidden border border-transparent ${hoverBorder} transition-[border-color,box-shadow] duration-300 flex flex-col cursor-pointer`}
              onClick={() => setSelectedDevice(device)}
            >
              <div className="p-6">
                <div className="flex justify-between items-start mb-6">
                  <div className={`bg-surface-container-highest p-3 rounded-lg ${statusTextColor(status)}`}>
                    <span className="material-symbols-outlined text-3xl">{iconForProtocol(device.protocol)}</span>
                  </div>
                  <div className={`flex items-center gap-2 px-3 py-1 rounded-full ${statusBgColor(status)}/10`}>
                    {(status === 'up' || status === 'ok') && <span className={`w-1.5 h-1.5 rounded-full animate-pulse ${statusBgColor(status)}`} />}
                    <span className={`text-[10px] font-bold uppercase tracking-widest ${statusTextColor(status)}`}>{status.toUpperCase()}</span>
                  </div>
                </div>
                <h3 className={`font-headline text-xl font-bold mb-1 ${hoverText} transition-colors`}>{device.name}</h3>
                <code className="text-on-surface-variant text-xs mb-4 block">{device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}{device.port > 0 && !['http','https'].includes(device.protocol) ? `:${device.port}` : ''}</code>
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Protocol</span><span className="font-bold">{device.protocol.toUpperCase()}</span></div>
                  <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Interval</span><span className="font-bold">{device.interval}s</span></div>
                  {metric && <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Response</span><span className="font-bold">{metric.responseTime ?? '-'}ms</span></div>}
                </div>
              </div>
              <div className="mt-auto bg-surface-container-high p-4 flex justify-between items-center">
                <span className="text-[10px] text-on-surface-variant uppercase">{metric ? new Date(metric.timestamp || metric.createdAt).toLocaleTimeString() : 'No data'}</span>
                <button 
                  onClick={(e) => { e.stopPropagation(); handleDelete(device); }} 
                  className="text-error text-[10px] font-bold uppercase tracking-widest hover:bg-error/10 px-2 py-1 rounded transition-colors"
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}

        {/* Add New Card */}
        <div onClick={() => setShowForm(true)} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setShowForm(true); } }} className="bg-surface-container-low rounded-xl border-2 border-dashed border-outline-variant/30 hover:border-primary/50 transition-[border-color] duration-300 flex flex-col items-center justify-center p-12 text-center group cursor-pointer min-h-[200px]">
          <div className="w-16 h-16 rounded-full bg-surface-container-high flex items-center justify-center mb-4 group-hover:bg-primary/10 transition-colors">
            <span className="material-symbols-outlined text-3xl text-outline group-hover:text-primary">add</span>
          </div>
          <h3 className="font-headline text-lg font-bold text-outline group-hover:text-on-surface uppercase tracking-widest">New Node</h3>
          <p className="text-on-surface-variant text-xs max-w-[120px] mx-auto mt-2">Deploy a new probe or add an existing node</p>
        </div>
      </div>
      {selectedDevice && (
        <DeviceModal 
          device={selectedDevice} 
          onClose={() => setSelectedDevice(null)} 
          onDeleted={() => {
            setSelectedDevice(null);
            load();
          }}
        />
      )}
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Device"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
      </>
      )}
    </div>
  );
}
