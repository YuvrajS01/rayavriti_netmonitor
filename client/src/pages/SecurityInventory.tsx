import { useCallback, useEffect, useMemo, useState } from 'react';
import { deleteDevice, getDevices, getLatestMetrics } from '../api/client';
import type { Device, Metric } from '../api/types';
import DeviceAddModal from '../components/DeviceAddModal';
import DeviceModal from '../components/DeviceModal';
import ConfirmDialog from '../components/ConfirmDialog';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';
import { statusBgColor, statusTextColor } from '../utils/colors';

type Profile = 'camera' | 'biometric';

const pageMeta: Record<Profile, { title: string; subtitle: string; icon: string; categories: string[] }> = {
  camera: { title: 'Camera Inventory', subtitle: 'IP cameras, NVRs, and DVRs monitored through management and RTSP checks.', icon: 'videocam', categories: ['camera', 'nvr', 'cctv'] },
  biometric: { title: 'Biometric Inventory', subtitle: 'Attendance terminals monitored through management and attendance services.', icon: 'fingerprint', categories: ['biometric'] },
};

function deviceStatus(device: Device, metrics: Map<number, Metric>) { return metrics.get(device.id)?.status || device.status || 'unknown'; }

export default function SecurityInventory({ profile }: { profile: Profile }) {
  const meta = pageMeta[profile];
  const [devices, setDevices] = useState<Device[]>([]);
  const [metrics, setMetrics] = useState<Map<number, Metric>>(new Map());
  const [loading, setLoading] = useState(true);
  const [addOpen, setAddOpen] = useState(false);
  const [selected, setSelected] = useState<Device | null>(null);
  const [deleting, setDeleting] = useState<Device | null>(null);

  const load = useCallback(async () => {
    try {
      const [deviceResponse, metricResponse] = await Promise.all([getDevices(), getLatestMetrics()]);
      setDevices(Array.isArray(deviceResponse.data) ? deviceResponse.data : []);
      setMetrics(new Map((Array.isArray(metricResponse.data) ? metricResponse.data : []).map((metric) => [metric.deviceId, metric])));
    } finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);

  const listed = useMemo(() => devices.filter((device) => device.protocol === profile || meta.categories.includes((device.deviceCategory || '').toLowerCase())), [devices, meta.categories, profile]);
  const online = listed.filter((device) => ['up', 'ok'].includes(deviceStatus(device, metrics))).length;
  const attention = listed.filter((device) => ['down', 'warning', 'degraded'].includes(deviceStatus(device, metrics))).length;

  const remove = async () => {
    if (!deleting) return;
    await deleteDevice(deleting.id);
    setDeleting(null);
    setSelected(null);
    void load();
  };

  return <div>
    <SectionHeader title={meta.title} subtitle={meta.subtitle} action={<Button icon="add_circle" onClick={() => setAddOpen(true)}>Add {profile === 'camera' ? 'Camera' : 'Terminal'}</Button>} />
    <div className="grid grid-cols-1 gap-4 md:grid-cols-3 mb-8"><StatCard label="Registered" value={listed.length} icon={meta.icon} /><StatCard label="Online" value={online} icon="check_circle" color="text-success" /><StatCard label="Needs attention" value={attention} icon="warning" color={attention ? 'text-warning' : 'text-on-surface'} /></div>
    {loading ? <div className="py-12 text-sm text-on-surface-variant">Loading inventory…</div> : listed.length === 0 ? <EmptyState icon={meta.icon} title={`No ${profile === 'camera' ? 'cameras' : 'biometric terminals'} yet`} description={`Add a ${profile === 'camera' ? 'camera or NVR' : 'terminal'} to begin independent service monitoring.`} action={<Button icon="add_circle" onClick={() => setAddOpen(true)}>Add device</Button>} /> : <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">{listed.map((device) => { const status = deviceStatus(device, metrics); const metric = metrics.get(device.id); return <button key={device.id} onClick={() => setSelected(device)} className="group rounded-lg border border-outline-variant/20 bg-surface-container-low p-5 text-left transition hover:-translate-y-0.5 hover:border-primary/40"><div className="flex items-start justify-between gap-3"><span className={`material-symbols-outlined text-3xl ${statusTextColor(status)}`}>{meta.icon}</span><span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold uppercase ${statusBgColor(status)}/10 ${statusTextColor(status)}`}>{status}</span></div><h2 className="mt-5 font-headline text-lg font-semibold">{device.name}</h2><p className="mt-1 text-xs text-on-surface-variant">{device.ipAddress}:{device.port || 80}</p><dl className="mt-5 grid grid-cols-2 gap-y-2 text-xs"><dt className="text-on-surface-variant">Vendor</dt><dd className="text-right font-semibold">{device.manufacturer || 'Unidentified'}</dd><dt className="text-on-surface-variant">Response</dt><dd className="text-right font-semibold">{metric?.responseTime ?? '—'} ms</dd></dl></button>; })}</div>}
    <DeviceAddModal open={addOpen} onClose={() => setAddOpen(false)} onAdded={() => void load()} initialProtocol={profile} />
    {selected && <DeviceModal device={selected} onClose={() => setSelected(null)} onDeleted={() => { setSelected(null); void load(); }} />}
    <ConfirmDialog open={!!deleting} title="Delete device" message={`Delete "${deleting?.name}"? This cannot be undone.`} confirmLabel="Delete" danger onConfirm={remove} onCancel={() => setDeleting(null)} />
  </div>;
}
