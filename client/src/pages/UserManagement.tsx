import Phase2Page from './Phase2Page';

export default function UserManagement() {
  return (
    <Phase2Page
      config={{
        title: 'User Management',
        subtitle: 'Review roles and configure scoped access for departments and operators.',
        icon: 'manage_accounts',
        path: '/users',
        primaryFields: ['username', 'role', 'email', 'enabled'],
      }}
    />
  );
}
