import { useState, useEffect, useCallback, type FormEvent } from 'react';
import { getDevices, getLatestMetrics, addDevice, deleteDevice } from '../api/client';
import type { Device, Metric } from '../api/types';
import DeviceModal from '../components/DeviceModal';

function statusOf(deviceId: number, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(deviceId)?.status || 'unknown';
}

function statusColor(status: string) {
  if (status === 'down') return { text: 'text-error', bg: 'bg-error', border: 'border-error/10' };
  if (status === 'warning' || status === 'degraded') return { text: 'text-amber-500', bg: 'bg-amber-500', border: 'border-amber-500/20' };
  if (status === 'unknown') return { text: 'text-outline', bg: 'bg-outline', border: 'border-outline-variant/20' };
  return { text: 'text-primary', bg: 'bg-primary', border: 'border-primary/20' };
}

function iconForProtocol(protocol: string) {
  if (protocol === 'ping' || protocol === 'icmp') return 'router';
  if (protocol === 'http' || protocol === 'https') return 'public';
  if (protocol === 'port' || protocol === 'tcp') return 'hub';
  if (protocol === 'system') return 'memory';
  if (protocol === 'snmp') return 'settings_input_antenna';
  return 'dns';
}

export default function Devices() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [metricsMap, setMetricsMap] = useState<Map<number, Metric>>(new Map());
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [formProtocol, setFormProtocol] = useState('https');
  const [formPort, setFormPort] = useState(443);
  const [snmpCommunity, setSnmpCommunity] = useState('public');
  const [snmpVersion, setSnmpVersion] = useState('2c');

  const load = useCallback(async () => {
    const [dRes, mRes] = await Promise.all([getDevices(), getLatestMetrics()]);
    setDevices(dRes.data || []);
    setMetricsMap(new Map((mRes.data || []).map((m) => [m.deviceId, m])));
  }, []);

  useEffect(() => { load(); }, [load]);

  const filtered = devices.filter((d) => {
    const status = statusOf(d.id, metricsMap);
    const text = `${d.name} ${d.ipAddress} ${d.protocol}`.toLowerCase();
    const matchSearch = !search || text.includes(search.toLowerCase());
    let matchStatus = true;
    if (statusFilter === 'up') matchStatus = status === 'up' || status === 'ok';
    else if (statusFilter === 'warning') matchStatus = status === 'warning' || status === 'degraded';
    else if (statusFilter === 'down') matchStatus = status === 'down';
    else if (statusFilter === 'unknown') matchStatus = status === 'unknown';
    return matchSearch && matchStatus;
  });

  const handleAdd = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const payload: Parameters<typeof addDevice>[0] = {
      name: fd.get('name') as string,
      host: fd.get('host') as string,
      protocol: formProtocol,
      port: formPort,
      interval: Number(fd.get('interval') || 60),
    };
    if (formProtocol === 'snmp') {
      payload.snmpCommunity = snmpCommunity.trim() || 'public';
      payload.snmpVersion = snmpVersion;
    }
    await addDevice(payload);
    e.currentTarget.reset();
    setShowForm(false);
    setFormProtocol('https');
    setFormPort(443);
    setSnmpCommunity('public');
    setSnmpVersion('2c');
    load();
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this device?')) return;
    await deleteDevice(id);
    load();
  };

  // Stats
  const total = devices.length;
  const up = devices.filter((d) => { const s = statusOf(d.id, metricsMap); return s === 'up' || s === 'ok'; }).length;
  const down = devices.filter((d) => statusOf(d.id, metricsMap) === 'down').length;
  const warn = devices.filter((d) => { const s = statusOf(d.id, metricsMap); return s === 'warning' || s === 'degraded'; }).length;

  return (
    <div>
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end justify-between mb-12 gap-6">
        <div>
          <h1 className="font-headline text-5xl font-black text-on-surface uppercase tracking-tight mb-2">My Devices</h1>
          <p className="text-on-surface-variant font-body max-w-xl">Centralized node management. Monitor real-time telemetry across your infrastructure.</p>
        </div>
        <button onClick={() => setShowForm(!showForm)} className="bg-primary text-on-primary font-headline font-bold py-4 px-8 rounded-none flex items-center gap-3 tracking-widest hover:brightness-110 active:scale-95 transition-all uppercase">
          <span className="material-symbols-outlined">add_circle</span>
          ADD DEVICE
        </button>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-12">
        <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-primary/30">
          <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">TOTAL NODES</p>
          <p className="font-headline text-3xl font-bold text-primary">{total}</p>
        </div>
        <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-primary">
          <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">ACTIVE (UP)</p>
          <p className="font-headline text-3xl font-bold text-primary">{up}</p>
        </div>
        <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-error">
          <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">CRITICAL (DOWN)</p>
          <p className="font-headline text-3xl font-bold text-error">{down}</p>
        </div>
        <div className="bg-surface-container-low p-6 rounded-xl border-l-2 border-amber-500">
          <p className="text-on-surface-variant text-[10px] uppercase tracking-[0.2em] mb-1">WARNINGS</p>
          <p className="font-headline text-3xl font-bold text-amber-500">{warn}</p>
        </div>
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
            <input
              name="port"
              type="number"
              min="1"
              max="65535"
              value={formPort}
              onChange={(e) => setFormPort(Number(e.target.value || 0))}
              placeholder="Port"
              className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none"
            />
            <select
              name="protocol"
              value={formProtocol}
              onChange={(e) => {
                const next = e.target.value;
                setFormProtocol(next);
                if (next === 'https') setFormPort(443);
                if (next === 'http') setFormPort(80);
                if (next === 'snmp') setFormPort(161);
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
            <button className="md:col-span-5 bg-primary text-on-primary rounded-lg px-4 py-2 text-xs font-bold tracking-widest uppercase hover:brightness-110 active:scale-95 transition-all">Add Device</button>
          </form>
        </div>
      )}

      {/* Device Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filtered.map((device) => {
          const status = statusOf(device.id, metricsMap);
          const sc = statusColor(status);
          const metric = metricsMap.get(device.id);
          return (
            <div 
              key={device.id} 
              className={`bg-surface-container-low rounded-xl group overflow-hidden border border-transparent hover:${sc.border} transition-all duration-300 flex flex-col cursor-pointer`}
              onClick={() => setSelectedDevice(device)}
            >
              <div className="p-6">
                <div className="flex justify-between items-start mb-6">
                  <div className={`bg-surface-container-highest p-3 rounded-lg ${sc.text}`}>
                    <span className="material-symbols-outlined text-3xl">{iconForProtocol(device.protocol)}</span>
                  </div>
                  <div className={`flex items-center gap-2 px-3 py-1 ${sc.bg}/10 rounded-full`}>
                    {(status === 'up' || status === 'ok') && <span className={`w-1.5 h-1.5 ${sc.bg} rounded-full animate-pulse`} />}
                    <span className={`text-[10px] font-bold ${sc.text} uppercase tracking-widest`}>{status.toUpperCase()}</span>
                  </div>
                </div>
                <h3 className={`font-headline text-xl font-bold mb-1 group-hover:${sc.text} transition-colors`}>{device.name}</h3>
                <code className="text-on-surface-variant text-xs mb-4 block">{device.ipAddress}:{device.port}</code>
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Protocol</span><span className="font-bold">{device.protocol.toUpperCase()}</span></div>
                  <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Interval</span><span className="font-bold">{device.interval}s</span></div>
                  {metric && <div className="flex justify-between"><span className="text-on-surface-variant uppercase tracking-widest">Response</span><span className="font-bold">{metric.responseTime}ms</span></div>}
                </div>
              </div>
              <div className="mt-auto bg-surface-container-high p-4 flex justify-between items-center">
                <span className="text-[10px] text-on-surface-variant uppercase">{metric ? new Date(metric.timestamp || metric.createdAt).toLocaleTimeString() : 'No data'}</span>
                <button 
                  onClick={(e) => { e.stopPropagation(); handleDelete(device.id); }} 
                  className="text-error text-[10px] font-bold uppercase tracking-widest hover:bg-error/10 px-2 py-1 rounded transition-colors"
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}

        {/* Add New Card */}
        <div onClick={() => setShowForm(true)} className="bg-surface-container-low rounded-xl border-2 border-dashed border-outline-variant/30 hover:border-primary/50 transition-all duration-300 flex flex-col items-center justify-center p-12 text-center group cursor-pointer min-h-[200px]">
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
    </div>
  );
}
