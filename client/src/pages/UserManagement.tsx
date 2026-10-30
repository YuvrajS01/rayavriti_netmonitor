import { useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createRole, createUser, deleteRole, deleteUser, getRoles, getUsers, updateRole, updateUser, type ManagedUser, type Role } from '../api/users';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';

const EMPTY_USERS: ManagedUser[] = [];
const EMPTY_ROLES: Role[] = [];

const PERMISSION_GROUPS = [
  {
    label: 'Devices',
    icon: 'devices',
    permissions: [
      { key: 'devices.read', label: 'View devices', desc: 'View device list and details' },
      { key: 'devices.write', label: 'Manage devices', desc: 'Add, edit, and configure devices' },
      { key: 'devices.delete', label: 'Delete devices', desc: 'Remove devices from monitoring' },
    ],
  },
  {
    label: 'Alerts',
    icon: 'warning',
    permissions: [
      { key: 'alerts.read', label: 'View alerts', desc: 'View active and historical alerts' },
      { key: 'alerts.create', label: 'Create alerts', desc: 'Manually create alerts' },
      { key: 'alerts.acknowledge', label: 'Acknowledge alerts', desc: 'Mark alerts as acknowledged' },
      { key: 'alerts.resolve', label: 'Resolve alerts', desc: 'Mark alerts as resolved' },
      { key: 'alert_rules.write', label: 'Manage alert rules', desc: 'Create and edit alert rules' },
    ],
  },
  {
    label: 'Incidents',
    icon: 'crisis_alert',
    permissions: [
      { key: 'incidents.write', label: 'Manage incidents', desc: 'Create, update, and resolve incidents' },
    ],
  },
  {
    label: 'Maintenance',
    icon: 'event_repeat',
    permissions: [
      { key: 'maintenance.write', label: 'Manage maintenance', desc: 'Create and edit maintenance windows' },
    ],
  },
  {
    label: 'Contacts & Notifications',
    icon: 'contacts',
    permissions: [
      { key: 'contacts.write', label: 'Manage contacts', desc: 'Add and edit escalation contacts' },
      { key: 'notifications.manage', label: 'Manage notifications', desc: 'Configure notification channels' },
    ],
  },
  {
    label: 'Reports',
    icon: 'analytics',
    permissions: [
      { key: 'reports.read', label: 'View reports', desc: 'Access reports and dashboards' },
      { key: 'reports.write', label: 'Manage reports', desc: 'Create and schedule reports' },
      { key: 'sla.manage', label: 'Manage SLA', desc: 'Configure SLA definitions' },
    ],
  },
  {
    label: 'ISP & Discovery',
    icon: 'router',
    permissions: [
      { key: 'discovery.execute', label: 'Run discovery scans', desc: 'Launch subnet scans and approve devices' },
      { key: 'capture.execute', label: 'Packet capture', desc: 'Start and stop packet captures' },
      { key: 'status_page.manage', label: 'Manage status page', desc: 'Configure public status page services' },
    ],
  },
  {
    label: 'System',
    icon: 'admin_panel_settings',
    permissions: [
      { key: 'settings.write', label: 'System settings', desc: 'Modify system configuration' },
      { key: 'users.manage', label: 'User management', desc: 'Manage users, roles, and permissions' },
      { key: 'import.execute', label: 'Bulk import', desc: 'Import devices via CSV' },
      { key: 'system.monitoring', label: 'System monitoring', desc: 'View system resource metrics' },
      { key: 'system.logs', label: 'System logs', desc: 'Access application logs' },
    ],
  },
];

const ALL_PERMISSIONS = PERMISSION_GROUPS.flatMap((g) => g.permissions.map((p) => p.key));

function timeAgo(ts: string | null): string {
  if (!ts) return 'Never';
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

export default function UserManagement() {
  const { addToast } = useToast();
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [tab, setTab] = useState<'users' | 'roles'>('users');
  const [editingUser, setEditingUser] = useState<ManagedUser | null>(null);
  const [userForm, setUserForm] = useState<Record<string, unknown>>({});
  const [showUserForm, setShowUserForm] = useState(false);
  const [userCreateForm, setUserCreateForm] = useState({ username: '', password: '', role: 'viewer', display_name: '', email: '', phone: '' });
  const [deleteUserTarget, setDeleteUserTarget] = useState<ManagedUser | null>(null);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [showRoleForm, setShowRoleForm] = useState(false);
  const [roleForm, setRoleForm] = useState({ name: '', display_name: '', description: '', permissions: [] as string[] });
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null);

  const usersQuery = useQuery({ queryKey: queryKeys.users.list(), queryFn: ({ signal }) => getUsers({ signal }) });
  const rolesQuery = useQuery({ queryKey: queryKeys.users.roles(), queryFn: ({ signal }) => getRoles({ signal }) });
  const loading = usersQuery.isPending || rolesQuery.isPending;
  const users = usersQuery.data?.data || EMPTY_USERS;
  const roles = rolesQuery.data?.data || EMPTY_ROLES;

  // Wraps an admin action with the shared submitting flag, toast and refresh.
  const adminMutation = useMutation({
    mutationFn: ({ run }: { run: () => Promise<unknown>; done: string; fail: string; after?: () => void }) => run(),
    onSuccess: (_res, { done, after }) => {
      addToast(done, 'success');
      after?.();
      return queryClient.invalidateQueries({ queryKey: queryKeys.users.all });
    },
    onError: (err, { fail }) => addToast(err instanceof Error ? err.message : fail, 'error'),
  });
  const submitting = adminMutation.isPending;

  const filteredUsers = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return users.filter((u) => !needle || u.username.toLowerCase().includes(needle) || u.display_name?.toLowerCase().includes(needle) || u.email?.toLowerCase().includes(needle));
  }, [users, search]);

  const filteredRoles = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return roles.filter((r) => !needle || r.name.toLowerCase().includes(needle) || r.display_name?.toLowerCase().includes(needle));
  }, [roles, search]);

  const stats = useMemo(() => ({
    totalUsers: users.length,
    enabledUsers: users.filter((u) => u.enabled).length,
    totalRoles: roles.length,
    customRoles: roles.filter((r) => !r.is_system).length,
  }), [users, roles]);

  const openEditUser = (u: ManagedUser) => {
    setEditingUser(u);
    setUserForm({ role: u.role, display_name: u.display_name, email: u.email, phone: u.phone, enabled: u.enabled });
  };

  const handleUserUpdate = () => {
    if (!editingUser) return;
    const target = editingUser;
    adminMutation.mutate({ run: () => updateUser(target.id, userForm), done: 'User updated', fail: 'Update failed', after: () => setEditingUser(null) });
  };

  const openCreateRole = () => {
    setEditingRole(null);
    setRoleForm({ name: '', display_name: '', description: '', permissions: [] });
    setShowRoleForm(true);
  };

  const openEditRole = (r: Role) => {
    setEditingRole(r);
    setRoleForm({ name: r.name, display_name: r.display_name || '', description: r.description || '', permissions: [...(r.permissions || [])] });
    setShowRoleForm(true);
  };

  const togglePermission = (perm: string) => {
    setRoleForm((prev) => ({
      ...prev,
      permissions: prev.permissions.includes(perm)
        ? prev.permissions.filter((p) => p !== perm)
        : [...prev.permissions, perm],
    }));
  };

  const toggleGroup = (groupPerms: string[]) => {
    setRoleForm((prev) => {
      const allSelected = groupPerms.every((p) => prev.permissions.includes(p));
      return {
        ...prev,
        permissions: allSelected
          ? prev.permissions.filter((p) => !groupPerms.includes(p))
          : [...new Set([...prev.permissions, ...groupPerms])],
      };
    });
  };

  const handleRoleSubmit = () => {
    const role = editingRole;
    adminMutation.mutate({
      run: () => role
        ? updateRole(role.id, {
            display_name: roleForm.display_name,
            description: roleForm.description,
            permissions: roleForm.permissions,
          })
        : createRole({
            name: roleForm.name,
            display_name: roleForm.display_name,
            description: roleForm.description,
            permissions: roleForm.permissions,
            is_system: false,
          }),
      done: role ? 'Role updated' : 'Role created',
      fail: 'Save failed',
      after: () => setShowRoleForm(false),
    });
  };

  const handleDeleteRole = () => {
    if (!deleteTarget) return;
    const role = deleteTarget;
    adminMutation.mutate({ run: () => deleteRole(role.id), done: 'Role deleted', fail: 'Delete failed', after: () => setDeleteTarget(null) });
  };

  const handleCreateUser = () => {
    if (!userCreateForm.username || !userCreateForm.password) {
      addToast('Username and password are required', 'error');
      return;
    }
    const body = userCreateForm;
    adminMutation.mutate({
      run: () => createUser(body),
      done: 'User created',
      fail: 'Create failed',
      after: () => {
        setShowUserForm(false);
        setUserCreateForm({ username: '', password: '', role: 'viewer', display_name: '', email: '', phone: '' });
      },
    });
  };

  const handleDeleteUser = () => {
    if (!deleteUserTarget) return;
    const user = deleteUserTarget;
    adminMutation.mutate({ run: () => deleteUser(user.id), done: 'User deleted', fail: 'Delete failed', after: () => setDeleteUserTarget(null) });
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="User Management"
        subtitle="Manage roles, permissions, and scoped access for operators."
        action={tab === 'roles' ? <Button icon="add" onClick={openCreateRole}>New Role</Button> : <Button icon="person_add" onClick={() => { setUserCreateForm({ username: '', password: '', role: 'viewer', display_name: '', email: '', phone: '' }); setShowUserForm(true); }}>New User</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Users" value={stats.totalUsers} icon="people" sparklineData={roles.map(role => users.filter(user => user.role === role.name).length)} />
        <StatCard label="Active" value={stats.enabledUsers} icon="check_circle" trend="up" sparklineData={users.map(user => user.enabled ? 1 : 0)} />
        <StatCard label="Roles" value={stats.totalRoles} icon="shield" sparklineData={roles.map(role => role.permissions.length)} />
        <StatCard label="Custom Roles" value={stats.customRoles} icon="tune" sparklineData={roles.map(role => role.is_system ? 0 : 1)} />
      </div>

      <Card variant="low" className="p-5"><div className="flex items-center justify-between mb-4"><div><h2 className="font-sans text-sm font-semibold uppercase tracking-wide">Role distribution</h2><p className="text-xs text-fg-muted mt-1">Active users by assigned role</p></div><span className="material-symbols-outlined text-primary">admin_panel_settings</span></div><div className="flex h-4 rounded-full overflow-hidden bg-inset">{roles.map((role, index) => { const count = users.filter(user => user.role === role.name).length; const color = `var(--color-chart-${index % 8 + 1})`; return <i key={role.id} title={`${role.display_name || role.name}: ${count}`} style={{ width: `${users.length ? count / users.length * 100 : 0}%`, background: color }} />; })}</div><div className="flex flex-wrap gap-x-4 gap-y-2 mt-3">{roles.map((role, index) => <span key={role.id} className="text-xs text-fg-muted"><i className="inline-block w-2 h-2 rounded-full mr-1.5" style={{ background: `var(--color-chart-${index % 8 + 1})` }} />{role.display_name || role.name} · {users.filter(user => user.role === role.name).length}</span>)}</div></Card>

      <div className="flex gap-1 bg-surface-1 rounded-lg p-1 w-fit">
        {(['users', 'roles'] as const).map((t) => (
          <button key={t} onClick={() => { setTab(t); setSearch(''); }} className={`px-4 py-2 text-xs font-sans font-semibold uppercase tracking-wide rounded-md transition-colors ${tab === t ? 'bg-primary text-on-primary' : 'text-fg-muted hover:text-fg'}`}>
            {t === 'users' ? 'Users' : 'Roles & Permissions'}
          </button>
        ))}
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-line flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">{tab === 'users' ? 'people' : 'shield'}</span>
            </div>
            <div>
              <h2 className="font-sans font-semibold text-lg">{tab === 'users' ? 'Users' : 'Roles & Permissions'}</h2>
              <p className="text-xs text-fg-muted uppercase tracking-wide">{tab === 'users' ? filteredUsers.length : filteredRoles.length} items</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading...</div>
        ) : tab === 'users' ? (
          filteredUsers.length === 0 ? (
            <EmptyState icon="people" title="No users" description="Users will appear here after registration." />
          ) : (
            <div className="divide-y divide-line">
              {filteredUsers.map((u) => (
                <article key={u.id} className="p-5 hover:bg-surface-1/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                    <div className="flex items-center gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-full bg-inset flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">person</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-sans font-semibold text-lg truncate">{u.display_name || u.username}</h3>
                          {!u.enabled && <span className="text-[10px] font-semibold uppercase px-2 py-0.5 rounded-full bg-line-strong/10 text-fg-subtle">Disabled</span>}
                        </div>
                        <p className="text-xs text-fg-muted mt-0.5">@{u.username} · {u.email || 'No email'}</p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 lg:max-w-lg flex-shrink-0">
                      <div>
                        <div className="text-[10px] text-fg-muted uppercase tracking-wide">Role</div>
                        <div className="text-sm font-semibold capitalize text-primary">{u.role?.replace(/_/g, ' ')}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-fg-muted uppercase tracking-wide">Last Login</div>
                        <div className="text-sm font-medium">{timeAgo(u.last_login_at)}</div>
                      </div>
                      <div className="flex items-end gap-2">
                        <button onClick={() => openEditUser(u)} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                        {u.role !== 'super_admin' && (
                          <button onClick={() => setDeleteUserTarget(u)} className="text-xs font-semibold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                        )}
                      </div>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          )
        ) : filteredRoles.length === 0 ? (
          <EmptyState icon="shield" title="No roles" description="Create a role to define a permission set." action={<Button icon="add" onClick={openCreateRole}>Create Role</Button>} />
        ) : (
          <div className="divide-y divide-line">
            {filteredRoles.map((r) => {
              const permCount = r.permissions?.length || 0;
              const groupsHit = new Set((r.permissions || []).map((p) => p.split('.')[0])).size;
              return (
                <article key={r.id} className="p-5 hover:bg-surface-1/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">shield</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-sans font-semibold text-lg truncate">{r.display_name || r.name}</h3>
                          {r.is_system && <span className="text-[10px] font-semibold uppercase px-2 py-0.5 rounded-full bg-primary/10 text-primary">System</span>}
                        </div>
                        {r.description && <p className="text-sm text-fg-muted mt-1">{r.description}</p>}
                        <div className="flex items-center gap-3 mt-2">
                          <span className="text-xs text-fg-muted">{permCount} permissions across {groupsHit} groups</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <button onClick={() => openEditRole(r)} className="text-xs font-semibold text-primary hover:bg-primary/10 px-3 py-1.5 rounded transition-colors">Edit</button>
                      {!r.is_system && (
                        <button onClick={() => setDeleteTarget(r)} className="text-xs font-semibold text-error hover:bg-error/10 px-3 py-1.5 rounded transition-colors">Delete</button>
                      )}
                    </div>
                  </div>
                  {permCount > 0 && (
                    <div className="flex flex-wrap gap-1 mt-3 pt-3 border-t border-line">
                      {(r.permissions || []).slice(0, 12).map((p) => (
                        <span key={p} className="text-[10px] font-mono px-2 py-0.5 rounded bg-inset text-fg-muted">{p}</span>
                      ))}
                      {permCount > 12 && <span className="text-[10px] text-fg-muted px-2 py-0.5">+{permCount - 12} more</span>}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        )}
      </Card>

      {editingUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setEditingUser(null)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">Edit User: {editingUser.username}</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Role</label>
                <select value={asText(userForm.role)} onChange={(e) => setUserForm({ ...userForm, role: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                  {roles.length > 0
                    ? roles.map((r) => <option key={r.id} value={r.name}>{r.display_name || r.name}</option>)
                    : <option value={asText(userForm.role, 'viewer')}>{asText(userForm.role, 'Viewer')}</option>}
                </select>
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Display Name</label>
                <input value={asText(userForm.display_name)} onChange={(e) => setUserForm({ ...userForm, display_name: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Email</label>
                <input value={asText(userForm.email)} onChange={(e) => setUserForm({ ...userForm, email: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" />
              </div>
              <div className="flex items-center gap-3">
                <input type="checkbox" checked={!!userForm.enabled} onChange={(e) => setUserForm({ ...userForm, enabled: e.target.checked })} className="accent-primary" id="user_enabled" />
                <label htmlFor="user_enabled" className="text-sm text-fg">Enabled</label>
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setEditingUser(null)} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleUserUpdate} disabled={submitting} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : 'Update'}</button>
            </div>
          </div>
        </div>
      )}

      {showRoleForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowRoleForm(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line flex items-center justify-between shrink-0">
              <div>
                <h2 className="font-sans text-lg font-semibold">{editingRole ? `Edit Role: ${editingRole.name}` : 'Create Role'}</h2>
                <p className="text-xs text-fg-muted mt-1">{roleForm.permissions.length} permissions selected</p>
              </div>
              <div className="flex gap-2">
                <button onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [...ALL_PERMISSIONS] }))} className="text-[10px] font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 px-3 py-1.5 rounded transition-colors">Select All</button>
                <button onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [] }))} className="text-[10px] font-semibold uppercase tracking-wide text-fg-muted hover:bg-surface-1 px-3 py-1.5 rounded transition-colors">Clear</button>
              </div>
            </div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              {!editingRole && (
                <div>
                  <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Role ID (unique key)</label>
                  <input value={roleForm.name} onChange={(e) => setRoleForm({ ...roleForm, name: e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, '_') })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg font-mono placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="network_operator" />
                </div>
              )}
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Display Name</label>
                <input value={roleForm.display_name} onChange={(e) => setRoleForm({ ...roleForm, display_name: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="Network Operator" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Description</label>
                <input value={roleForm.description} onChange={(e) => setRoleForm({ ...roleForm, description: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="Can view and acknowledge alerts" />
              </div>

              <div className="pt-2">
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-3">Permissions</label>
                <div className="space-y-4">
                  {PERMISSION_GROUPS.map((group) => {
                    const groupPerms = group.permissions.map((p) => p.key);
                    const selectedCount = groupPerms.filter((p) => roleForm.permissions.includes(p)).length;
                    const allSelected = selectedCount === groupPerms.length;
                    const someSelected = selectedCount > 0 && !allSelected;
                    return (
                      <div key={group.label} className="bg-inset rounded-lg p-4">
                        <div className="flex items-center gap-3 mb-3">
                          <input
                            type="checkbox"
                            checked={allSelected}
                            ref={(el) => { if (el) el.indeterminate = someSelected; }}
                            onChange={() => toggleGroup(groupPerms)}
                            className="accent-primary"
                            id={`grp_${group.label}`}
                          />
                          <span className="material-symbols-outlined text-sm text-primary">{group.icon}</span>
                          <label htmlFor={`grp_${group.label}`} className="font-sans font-semibold text-sm cursor-pointer">{group.label}</label>
                          <span className="text-[10px] text-fg-muted ml-auto">{selectedCount}/{groupPerms.length}</span>
                        </div>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 ml-8">
                          {group.permissions.map((perm) => (
                            <label key={perm.key} className="flex items-start gap-2 cursor-pointer group">
                              <input
                                type="checkbox"
                                checked={roleForm.permissions.includes(perm.key)}
                                onChange={() => togglePermission(perm.key)}
                                className="accent-primary mt-0.5"
                              />
                              <div>
                                <span className="text-sm text-fg group-hover:text-fg transition-colors">{perm.label}</span>
                                <span className="block text-[11px] text-fg-muted">{perm.desc}</span>
                              </div>
                            </label>
                          ))}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowRoleForm(false)} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleRoleSubmit} disabled={submitting || (!editingRole && !roleForm.name)} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editingRole ? 'Update Role' : 'Create Role'}</button>
            </div>
          </div>
        </div>
      )}

      {showUserForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowUserForm(false)}>
          <div className="bg-surface-1 border border-line rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-line shrink-0"><h2 className="font-sans text-lg font-semibold">Create User</h2></div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Username *</label>
                <input value={userCreateForm.username} onChange={(e) => setUserCreateForm({ ...userCreateForm, username: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="john.doe" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Password *</label>
                <input type="password" value={userCreateForm.password} onChange={(e) => setUserCreateForm({ ...userCreateForm, password: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="Minimum 6 characters" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Role</label>
                <select value={userCreateForm.role} onChange={(e) => setUserCreateForm({ ...userCreateForm, role: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg outline-none focus:ring-1 focus:ring-primary">
                  {roles.length > 0
                    ? roles.map((r) => <option key={r.id} value={r.name}>{r.display_name || r.name}</option>)
                    : ['viewer', 'dept_admin', 'network_admin', 'admin', 'super_admin'].map((r) => <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>)}
                </select>
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Display Name</label>
                <input value={userCreateForm.display_name} onChange={(e) => setUserCreateForm({ ...userCreateForm, display_name: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="John Doe" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Email</label>
                <input type="email" value={userCreateForm.email} onChange={(e) => setUserCreateForm({ ...userCreateForm, email: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="john@example.com" />
              </div>
              <div>
                <label className="text-[10px] text-fg-muted uppercase tracking-wide block mb-1">Phone</label>
                <input value={userCreateForm.phone} onChange={(e) => setUserCreateForm({ ...userCreateForm, phone: e.target.value })} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="+1 555 123 4567" />
              </div>
            </div>
            <div className="flex border-t border-line shrink-0">
              <button onClick={() => setShowUserForm(false)} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-fg-muted hover:bg-surface-1 transition-colors">Cancel</button>
              <div className="w-px bg-line/20" />
              <button onClick={handleCreateUser} disabled={submitting || !userCreateForm.username || !userCreateForm.password} className="flex-1 py-3 text-xs font-sans font-semibold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Creating...' : 'Create User'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteUserTarget} title="Delete User" message={`Delete user "${deleteUserTarget?.username}"? This action cannot be undone.`} confirmLabel="Delete" danger onConfirm={handleDeleteUser} onCancel={() => setDeleteUserTarget(null)} />

      <ConfirmDialog open={!!deleteTarget} title="Delete Role" message={`Delete "${deleteTarget?.display_name || deleteTarget?.name}"? Users with this role will lose its permissions.`} confirmLabel="Delete" danger onConfirm={handleDeleteRole} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
