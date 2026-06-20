import Phase2Page from './Phase2Page';

export default function ISP() {
  return (
    <Phase2Page
      config={{
        title: 'ISP Dashboard',
        subtitle: 'Track provider circuits, gateway health, bandwidth contracts, and SLA evidence.',
        icon: 'router',
        path: '/isp-links',
        primaryFields: ['provider', 'gateway_ip', 'bandwidth_mbps', 'sla_uptime_percent'],
        quickCreate: { name: 'Primary ISP', provider: 'Provider', gateway_ip: '8.8.8.8', monitoring_interval_seconds: 10, enabled: true },
      }}
    />
  );
}
