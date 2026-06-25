import { useState, useEffect, type FormEvent } from 'react';
import { addDevice } from '../api/client';
import { listPhase2, type Phase2Row } from '../api/phase2';
import Button from './ui/Button';
import { useToast } from './ui/useToast';

interface Props {
  open: boolean;
  onClose: () => void;
  onAdded: () => void;
}

export default function DeviceAddModal({ open, onClose, onAdded }: Props) {
  const { addToast } = useToast();
  const [protocol, setProtocol] = useState('https');
  const [port, setPort] = useState(443);
  const [snmpCommunity, setSnmpCommunity] = useState('public');
  const [snmpVersion, setSnmpVersion] = useState('2c');
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [locations, setLocations] = useState<Phase2Row[]>([]);

  useEffect(() => {
    if (open) {
      listPhase2('/locations').then((res) => setLocations(res.data || [])).catch(() => setLocations([]));
    }
  }, [open]);

  if (!open) return null;

  const validate = (fd: FormData): boolean => {
    const newErrors: Record<string, string> = {};
    const name = fd.get('name') as string;
    const host = fd.get('host') as string;
    if (!name?.trim()) newErrors.name = 'Name is required';
    if (!host?.trim()) newErrors.host = 'Host/IP is required';
    if (protocol === 'port') {
      const p = Number(fd.get('port'));
      if (!p || p < 1 || p > 65535) newErrors.port = 'Valid port required (1-65535)';
    }
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    if (!validate(fd)) return;

    setSubmitting(true);
    try {
      const payload: Parameters<typeof addDevice>[0] = {
        name: (fd.get('name') as string).trim(),
        ipAddress: (fd.get('host') as string).trim(),
        protocol,
        port,
        interval: Number(fd.get('interval') || 60),
      };
      const locVal = fd.get('locationId') as string;
      if (locVal) payload.locationId = Number(locVal);
      if (protocol === 'snmp') {
        payload.snmpCommunity = snmpCommunity.trim() || 'public';
        payload.snmpVersion = snmpVersion;
      }
      await addDevice(payload);
      addToast('Device added successfully', 'success');
      onAdded();
      onClose();
    } catch {
      addToast('Failed to add device', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const inputClass = 'bg-surface-container-highest border rounded-lg px-3 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary transition-colors';
  const borderClass = (field: string) => errors[field] ? 'border-error/50' : 'border-outline-variant/20';

  return (
    <div className="fixed inset-0 z-[90] flex items-center justify-center p-4 pt-20" role="dialog" aria-modal="true" aria-label="Add new device">
      <div className="absolute inset-0 bg-black/60 " onClick={onClose} />
      <div className="relative bg-surface-container-low rounded-lg border border-outline-variant/20 w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between p-6 border-b border-outline-variant/20 shrink-0">
          <div>
            <h2 className="font-headline text-xl font-bold text-on-surface uppercase tracking-wide">New Device</h2>
            <p className="text-xs text-on-surface-variant mt-1">Add a node to your monitoring network</p>
          </div>
          <button onClick={onClose} className="material-symbols-outlined text-on-surface-variant hover:text-on-surface transition-colors" aria-label="Close">
            close
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Device Name *</label>
            <input name="name" required placeholder="e.g. Core Router 01" className={`${inputClass} w-full ${borderClass('name')}`} />
            {errors.name && <p className="text-error text-[10px] mt-1">{errors.name}</p>}
          </div>

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Host / IP Address *</label>
            <input name="host" required placeholder="e.g. 192.168.1.1 or example.com" className={`${inputClass} w-full ${borderClass('host')}`} />
            {errors.host && <p className="text-error text-[10px] mt-1">{errors.host}</p>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Protocol</label>
              <select
                name="protocol"
                value={protocol}
                onChange={(e) => {
                  const next = e.target.value;
                  setProtocol(next);
                  if (next === 'https') setPort(443);
                  if (next === 'http') setPort(80);
                  if (next === 'snmp') setPort(161);
                  if (next === 'ping') setPort(0);
                  if (next === 'port') setPort(0);
                }}
                className={`${inputClass} w-full cursor-pointer`}
              >
                <option value="https">HTTPS</option>
                <option value="http">HTTP</option>
                <option value="ping">Ping (ICMP)</option>
                <option value="port">TCP Port</option>
                <option value="system">System</option>
                <option value="snmp">SNMP</option>
              </select>
            </div>
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Port</label>
              {['https', 'http', 'snmp', 'port'].includes(protocol) ? (
                <>
                  <input
                    name="port"
                    type="number"
                    min="1"
                    max="65535"
                    value={port || ''}
                    onChange={(e) => setPort(Number(e.target.value || 0))}
                    placeholder={protocol === 'port' ? 'Port to check' : 'Port'}
                    required={protocol === 'port'}
                    className={`${inputClass} w-full ${borderClass('port')}`}
                  />
                  {errors.port && <p className="text-error text-[10px] mt-1">{errors.port}</p>}
                </>
              ) : (
                <div className={`${inputClass} w-full text-on-surface-variant`}>N/A</div>
              )}
            </div>
          </div>

          {protocol === 'snmp' && (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">SNMP Community</label>
                <input
                  value={snmpCommunity}
                  onChange={(e) => setSnmpCommunity(e.target.value)}
                  placeholder="Community string"
                  className={`${inputClass} w-full`}
                />
              </div>
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">SNMP Version</label>
                <select value={snmpVersion} onChange={(e) => setSnmpVersion(e.target.value)} className={`${inputClass} w-full cursor-pointer`}>
                  <option value="2c">v2c</option>
                  <option value="1">v1</option>
                </select>
              </div>
            </div>
          )}

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Check Interval (seconds)</label>
            <input name="interval" type="number" min="10" defaultValue={60} className={`${inputClass} w-full`} />
          </div>

          {locations.length > 0 && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wide text-on-surface-variant mb-1.5">Location</label>
              <select name="locationId" defaultValue="" className={`${inputClass} w-full cursor-pointer`}>
                <option value="">Unassigned</option>
                {locations.map((loc) => (
                  <option key={Number(loc.id)} value={String(loc.id)}>
                    {String(loc.name)}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div className="flex gap-3 pt-2">
            <Button type="submit" icon="add_circle" disabled={submitting}>
              {submitting ? 'ADDING...' : 'ADD DEVICE'}
            </Button>
            <Button type="button" variant="secondary" onClick={onClose}>
              CANCEL
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
