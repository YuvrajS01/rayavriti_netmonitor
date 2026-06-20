import Phase2Page from './Phase2Page';

export default function StatusPageAdmin() {
  return (
    <Phase2Page
      config={{
        title: 'Status Page Admin',
        subtitle: 'Configure public services and incident announcements for the standalone status page.',
        icon: 'public',
        path: '/status-page/services',
        primaryFields: ['group_name', 'aggregation', 'show_uptime', 'enabled'],
        quickCreate: { name: 'Campus DNS', group_name: 'Core Infrastructure', aggregation: 'any_down', show_uptime: true, enabled: true },
      }}
    />
  );
}
