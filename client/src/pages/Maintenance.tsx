import Phase2Page from './Phase2Page';

export default function Maintenance() {
  return (
    <Phase2Page
      config={{
        title: 'Maintenance Calendar',
        subtitle: 'Plan one-time and recurring windows that suppress alerts and notifications.',
        icon: 'event_repeat',
        path: '/maintenance',
        primaryFields: ['scope_type', 'scope_value', 'schedule_type', 'enabled'],
        quickCreate: { name: 'Sunday Lab Shutdown', scope_type: 'global', scope_value: '*', schedule_type: 'recurring', recurrence_rule: 'FREQ=WEEKLY;BYDAY=SU', enabled: true },
      }}
    />
  );
}
