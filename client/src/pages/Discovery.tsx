import Phase2Page from './Phase2Page';

export default function Discovery() {
  return (
    <Phase2Page
      config={{
        title: 'Discovery Dashboard',
        subtitle: 'Launch subnet scans, review findings, and approve discovered devices.',
        icon: 'travel_explore',
        path: '/discovery/jobs',
        primaryFields: ['subnet', 'scan_type', 'status', 'devices_found'],
        quickCreate: { subnet: '10.0.0.0/24', scan_type: 'ping_only', status: 'running' },
      }}
    />
  );
}
