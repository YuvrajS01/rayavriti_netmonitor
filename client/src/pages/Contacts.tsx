import Phase2Page from './Phase2Page';

export default function Contacts() {
  return (
    <Phase2Page
      config={{
        title: 'Contact Directory',
        subtitle: 'Maintain owners, escalation contacts, preferred channels, and quiet hours.',
        icon: 'contacts',
        path: '/contacts',
        primaryFields: ['designation', 'department', 'email', 'preferred_channel'],
        quickCreate: { name: 'New Contact', preferred_channel: 'email', notification_enabled: true, enabled: true },
      }}
    />
  );
}
