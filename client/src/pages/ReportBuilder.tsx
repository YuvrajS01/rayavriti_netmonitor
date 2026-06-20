import Phase2Page from './Phase2Page';

export default function ReportBuilder() {
  return (
    <Phase2Page
      config={{
        title: 'Report Builder',
        subtitle: 'Create scheduled reports, configure recipients, and review generated archives.',
        icon: 'summarize',
        path: '/reports/scheduled',
        primaryFields: ['report_type', 'format', 'schedule_cron', 'enabled'],
        quickCreate: { name: 'Weekly IT Report', report_type: 'health_summary', format: 'pdf', schedule_cron: '0 9 * * 1', recipients: [], enabled: true },
      }}
    />
  );
}
