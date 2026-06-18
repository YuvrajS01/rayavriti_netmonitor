export function iconForProtocol(protocol: string): string {
  if (protocol === 'ping' || protocol === 'icmp') return 'router';
  if (protocol === 'http' || protocol === 'https') return 'public';
  if (protocol === 'port' || protocol === 'tcp') return 'hub';
  if (protocol === 'system') return 'memory';
  if (protocol === 'snmp') return 'settings_input_antenna';
  return 'dns';
}

export function sensorIconForProtocol(protocol: string): string {
  if (protocol === 'ping' || protocol === 'icmp') return 'speed';
  if (protocol === 'http' || protocol === 'https') return 'public';
  if (protocol === 'port' || protocol === 'tcp') return 'hub';
  if (protocol === 'system') return 'data_usage';
  if (protocol === 'snmp') return 'settings_input_antenna';
  return 'sensors';
}
