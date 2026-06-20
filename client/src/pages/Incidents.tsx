import Phase2Page from './Phase2Page';

export default function Incidents() {
  return (
    <Phase2Page
      config={{
        title: 'Incident Manager',
        subtitle: 'Track investigation, assignment, resolution, timelines, and SLA breach state.',
        icon: 'crisis_alert',
        path: '/incidents',
        primaryFields: ['severity', 'status', 'location_id', 'affected_device_count'],
        quickCreate: { title: 'Manual incident', severity: 'minor', status: 'open', source: 'manual', affected_device_count: 0 },
      }}
    />
  );
}
