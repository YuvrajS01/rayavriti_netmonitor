import { useState, type FormEvent } from 'react';
import { addDevice } from '../api/client';
import type { ResourceRow } from '../api/resources';
import type { SNMPv3Config, SNMPv3SecurityLevel } from '../api/types';
import { useResourceList } from '../hooks/useResources';
import Button from './ui/Button';
import Dialog from './ui/Dialog';
import { useToast } from './ui/useToast';
import { asText } from '../utils/formatters';

interface Props {
  open: boolean;
  onClose: () => void;
  onAdded: () => void;
  initialProtocol?: 'camera' | 'biometric';
}

const EMPTY_LOCATIONS: ResourceRow[] = [];
const SNMP_AUTH_PROTOCOLS = ['SHA256', 'SHA512', 'SHA384', 'SHA224', 'SHA', 'MD5'];
const SNMP_PRIV_PROTOCOLS = ['AES', 'AES256', 'AES192', 'AES256C', 'AES192C', 'DES'];

export default function DeviceAddModal({ open, onClose, onAdded, initialProtocol }: Props) {
  const { addToast } = useToast();
  const [protocol, setProtocol] = useState(initialProtocol || 'https');
  const [port, setPort] = useState(initialProtocol ? 80 : 443);
  const [snmpCommunity, setSnmpCommunity] = useState('public');
  const [snmpVersion, setSnmpVersion] = useState('2c');
  const [securityLevel, setSecurityLevel] = useState<SNMPv3SecurityLevel>('authPriv');
  const [securityCategory, setSecurityCategory] = useState<'camera' | 'nvr' | 'biometric'>('camera');
  const [submitting, setSubmitting] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const locations = useResourceList('/locations', { enabled: open }).data ?? EMPTY_LOCATIONS;
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
    if (protocol === 'snmp' && snmpVersion === '3') {
      if (!asText(fd.get('securityName')).trim()) newErrors.securityName = 'User name is required';
      if (securityLevel !== 'noAuthNoPriv' && asText(fd.get('authPassphrase')).length < 8) newErrors.authPassphrase = 'At least 8 characters';
      if (securityLevel === 'authPriv' && asText(fd.get('privPassphrase')).length < 8) newErrors.privPassphrase = 'At least 8 characters';
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
	  if (protocol === 'camera' || protocol === 'biometric') {
		const monitorConfig: Record<string, string | number> = {
		  managementScheme: asText(fd.get('managementScheme')) || 'http',
		  managementPath: asText(fd.get('managementPath')) || '/',
		};
		if (protocol === 'camera') {
		  monitorConfig.rtspPort = Number(fd.get('rtspPort') || 554);
		  monitorConfig.rtspPath = asText(fd.get('rtspPath')) || '/';
		} else {
		  monitorConfig.attendancePort = Number(fd.get('attendancePort') || 4370);
		}
		payload.deviceCategory = protocol === 'camera' ? securityCategory : 'biometric';
		payload.monitorConfig = monitorConfig;
	  }
      const locVal = fd.get('locationId') as string;
      if (locVal) payload.locationId = Number(locVal);
      if (protocol === 'snmp') {
        payload.snmpVersion = snmpVersion;
        if (snmpVersion === '3') {
          const v3: SNMPv3Config = {
            securityName: asText(fd.get('securityName')).trim(),
            securityLevel,
            contextName: asText(fd.get('contextName')).trim() || undefined,
          };
          if (securityLevel !== 'noAuthNoPriv') {
            v3.authProtocol = asText(fd.get('authProtocol'));
            v3.authPassphrase = asText(fd.get('authPassphrase'));
          }
          if (securityLevel === 'authPriv') {
            v3.privProtocol = asText(fd.get('privProtocol'));
            v3.privPassphrase = asText(fd.get('privPassphrase'));
          }
          payload.snmpV3 = v3;
        } else {
          payload.snmpCommunity = snmpCommunity.trim() || 'public';
        }
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

  const inputClass = 'bg-inset border rounded-md px-3 py-2.5 text-sm text-fg outline-none focus:border-primary transition-colors';
  const borderClass = (field: string) => errors[field] ? 'border-error' : 'border-line';

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="New device"
      description="Add a node to your monitoring network"
      closeOnBackdrop={false}
      footer={
        <>
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" form="device-add-form" icon="add_circle" disabled={submitting}>
            {submitting ? 'Adding…' : 'Add device'}
          </Button>
        </>
      }
    >
        <form id="device-add-form" onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="device-name" className="block text-xs font-semibold text-fg-muted mb-1.5">Device name *</label>
            <input id="device-name" name="name" required placeholder="e.g. Core Router 01" aria-invalid={errors.name ? true : undefined} aria-describedby={errors.name ? 'device-name-error' : undefined} className={`${inputClass} w-full ${borderClass('name')}`} />
            {errors.name && <p id="device-name-error" className="text-error text-xs mt-1">{errors.name}</p>}
          </div>

          <div>
            <label htmlFor="device-host" className="block text-xs font-semibold text-fg-muted mb-1.5">Host / IP address *</label>
            <input id="device-host" name="host" required placeholder="e.g. 192.168.1.1 or example.com" aria-invalid={errors.host ? true : undefined} aria-describedby={errors.host ? 'device-host-error' : undefined} className={`${inputClass} w-full ${borderClass('host')}`} />
            {errors.host && <p id="device-host-error" className="text-error text-xs mt-1">{errors.host}</p>}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label htmlFor="device-protocol" className="block text-xs font-semibold text-fg-muted mb-1.5">Protocol</label>
              <select
                id="device-protocol"
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
				  if (next === 'camera' || next === 'biometric') setPort(80);
                }}
                className={`${inputClass} w-full cursor-pointer`}
              >
                <option value="https">HTTPS</option>
                <option value="http">HTTP</option>
                <option value="ping">Ping (ICMP)</option>
                <option value="port">TCP Port</option>
                <option value="system">System</option>
                <option value="snmp">SNMP</option>
				<option value="camera">CCTV Camera / NVR</option>
				<option value="biometric">Biometric terminal</option>
              </select>
            </div>
            <div>
              <label htmlFor="device-port" className="block text-xs font-semibold text-fg-muted mb-1.5">Port</label>
			  {['https', 'http', 'snmp', 'port', 'camera', 'biometric'].includes(protocol) ? (
                <>
                  <input
                    id="device-port"
                    name="port"
                    type="number"
                    min="1"
                    max="65535"
                    value={port || ''}
                    onChange={(e) => setPort(Number(e.target.value || 0))}
                    placeholder={protocol === 'port' ? 'Port to check' : 'Port'}
                    required={protocol === 'port'}
                    aria-invalid={errors.port ? true : undefined}
                    aria-describedby={errors.port ? 'device-port-error' : undefined}
                    className={`${inputClass} w-full font-mono ${borderClass('port')}`}
                  />
                  {errors.port && <p id="device-port-error" className="text-error text-xs mt-1">{errors.port}</p>}
                </>
              ) : (
                <div className={`${inputClass} w-full border-line text-fg-muted`} id="device-port" aria-label="Port: not applicable">N/A</div>
              )}
            </div>
          </div>

          {protocol === 'snmp' && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="device-snmp-version" className="block text-xs font-semibold text-fg-muted mb-1.5">SNMP version</label>
                  <select id="device-snmp-version" value={snmpVersion} onChange={(e) => setSnmpVersion(e.target.value)} className={`${inputClass} w-full cursor-pointer`}>
                    <option value="2c">v2c</option>
                    <option value="3">v3</option>
                    <option value="1">v1</option>
                  </select>
                </div>
                {snmpVersion === '3' ? (
                  <div>
                    <label htmlFor="device-snmp-level" className="block text-xs font-semibold text-fg-muted mb-1.5">Security level</label>
                    <select id="device-snmp-level" value={securityLevel} onChange={(e) => setSecurityLevel(e.target.value as SNMPv3SecurityLevel)} className={`${inputClass} w-full cursor-pointer`}>
                      <option value="authPriv">Auth and privacy</option>
                      <option value="authNoPriv">Auth, no privacy</option>
                      <option value="noAuthNoPriv">No auth</option>
                    </select>
                  </div>
                ) : (
                  <div>
                    <label htmlFor="device-snmp-community" className="block text-xs font-semibold text-fg-muted mb-1.5">SNMP community</label>
                    <input
                      id="device-snmp-community"
                      value={snmpCommunity}
                      onChange={(e) => setSnmpCommunity(e.target.value)}
                      placeholder="Community string"
                      className={`${inputClass} w-full border-line`}
                    />
                  </div>
                )}
              </div>
              {snmpVersion === '3' && (
                <>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label htmlFor="device-snmp-user" className="block text-xs font-semibold text-fg-muted mb-1.5">User name *</label>
                      <input id="device-snmp-user" name="securityName" maxLength={32} autoComplete="off" aria-invalid={errors.securityName ? true : undefined} aria-describedby={errors.securityName ? 'device-snmp-user-error' : undefined} className={`${inputClass} w-full ${borderClass('securityName')}`} />
                      {errors.securityName && <p id="device-snmp-user-error" className="text-error text-xs mt-1">{errors.securityName}</p>}
                    </div>
                    <div>
                      <label htmlFor="device-snmp-context" className="block text-xs font-semibold text-fg-muted mb-1.5">Context name</label>
                      <input id="device-snmp-context" name="contextName" placeholder="Optional" autoComplete="off" className={`${inputClass} w-full border-line`} />
                    </div>
                  </div>
                  {securityLevel !== 'noAuthNoPriv' && (
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label htmlFor="device-snmp-auth-proto" className="block text-xs font-semibold text-fg-muted mb-1.5">Auth protocol</label>
                        <select id="device-snmp-auth-proto" name="authProtocol" className={`${inputClass} w-full cursor-pointer border-line`}>
                          {SNMP_AUTH_PROTOCOLS.map((p) => <option key={p} value={p}>{p.replace(/^SHA(\d+)$/, 'SHA-$1')}</option>)}
                        </select>
                      </div>
                      <div>
                        <label htmlFor="device-snmp-auth-pass" className="block text-xs font-semibold text-fg-muted mb-1.5">Auth passphrase *</label>
                        <input id="device-snmp-auth-pass" name="authPassphrase" type="password" autoComplete="new-password" aria-invalid={errors.authPassphrase ? true : undefined} aria-describedby={errors.authPassphrase ? 'device-snmp-auth-pass-error' : undefined} className={`${inputClass} w-full ${borderClass('authPassphrase')}`} />
                        {errors.authPassphrase && <p id="device-snmp-auth-pass-error" className="text-error text-xs mt-1">{errors.authPassphrase}</p>}
                      </div>
                    </div>
                  )}
                  {securityLevel === 'authPriv' && (
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label htmlFor="device-snmp-priv-proto" className="block text-xs font-semibold text-fg-muted mb-1.5">Privacy protocol</label>
                        <select id="device-snmp-priv-proto" name="privProtocol" className={`${inputClass} w-full cursor-pointer border-line`}>
                          {SNMP_PRIV_PROTOCOLS.map((p) => <option key={p} value={p}>{p.replace(/^AES(\d+)(C?)$/, 'AES-$1$2').replace(/^AES$/, 'AES-128')}</option>)}
                        </select>
                      </div>
                      <div>
                        <label htmlFor="device-snmp-priv-pass" className="block text-xs font-semibold text-fg-muted mb-1.5">Privacy passphrase *</label>
                        <input id="device-snmp-priv-pass" name="privPassphrase" type="password" autoComplete="new-password" aria-invalid={errors.privPassphrase ? true : undefined} aria-describedby={errors.privPassphrase ? 'device-snmp-priv-pass-error' : undefined} className={`${inputClass} w-full ${borderClass('privPassphrase')}`} />
                        {errors.privPassphrase && <p id="device-snmp-priv-pass-error" className="text-error text-xs mt-1">{errors.privPassphrase}</p>}
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
          )}

		  {(protocol === 'camera' || protocol === 'biometric') && (
			<div className="border-y border-line py-4 space-y-4">
			  <div className="flex items-center gap-2 text-fg">
				<span className="material-symbols-outlined text-lg text-fg-muted" aria-hidden="true">{protocol === 'camera' ? 'videocam' : 'fingerprint'}</span>
				<h3 className="text-sm font-semibold">{protocol === 'camera' ? 'CCTV profile' : 'Biometric profile'}</h3>
			  </div>
			  {protocol === 'camera' && <div>
				<label htmlFor="security-category" className="block text-xs font-semibold text-fg-muted mb-1.5">Device type</label>
				<select id="security-category" value={securityCategory} onChange={(e) => setSecurityCategory(e.target.value as 'camera' | 'nvr')} className={`${inputClass} w-full cursor-pointer`}>
				  <option value="camera">IP Camera</option><option value="nvr">NVR / DVR</option>
				</select>
			  </div>}
			  <div className="grid grid-cols-2 gap-4">
				<div><label htmlFor="management-scheme" className="block text-xs font-semibold text-fg-muted mb-1.5">Management protocol</label><select id="management-scheme" name="managementScheme" className={`${inputClass} w-full`}><option value="http">HTTP</option><option value="https">HTTPS</option></select></div>
				<div><label htmlFor="management-path" className="block text-xs font-semibold text-fg-muted mb-1.5">Management path</label><input id="management-path" name="managementPath" defaultValue="/" className={`${inputClass} w-full`} /></div>
			  </div>
			  {protocol === 'camera' ? <div className="grid grid-cols-2 gap-4"><div><label htmlFor="rtsp-port" className="block text-xs font-semibold text-fg-muted mb-1.5">RTSP port</label><input id="rtsp-port" name="rtspPort" type="number" min="1" max="65535" defaultValue={554} className={`${inputClass} w-full`} /></div><div><label htmlFor="rtsp-path" className="block text-xs font-semibold text-fg-muted mb-1.5">RTSP path</label><input id="rtsp-path" name="rtspPath" defaultValue="/" className={`${inputClass} w-full`} /></div></div> : <div><label htmlFor="attendance-port" className="block text-xs font-semibold text-fg-muted mb-1.5">Attendance port</label><input id="attendance-port" name="attendancePort" type="number" min="1" max="65535" defaultValue={4370} className={`${inputClass} w-full`} /><p className="mt-1 text-xs text-fg-muted">4370 is the standard ZKTeco-compatible terminal port.</p></div>}
			</div>
		  )}

          <div>
            <label htmlFor="device-interval" className="block text-xs font-semibold text-fg-muted mb-1.5">Check interval (seconds)</label>
            <input id="device-interval" name="interval" type="number" min="10" defaultValue={60} className={`${inputClass} w-full`} />
          </div>

          {locations.length > 0 && (
            <div>
              <label htmlFor="device-location" className="block text-xs font-semibold text-fg-muted mb-1.5">Location</label>
              <select id="device-location" name="locationId" defaultValue="" className={`${inputClass} w-full cursor-pointer`}>
                <option value="">Unassigned</option>
                {locations.map((loc) => (
                  <option key={Number(loc.id)} value={String(loc.id)}>
                    {String(loc.name)}
                  </option>
                ))}
              </select>
            </div>
          )}

        </form>
    </Dialog>
  );
}
