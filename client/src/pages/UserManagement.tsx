import { useId, useState, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createRole, createUser, deleteRole, deleteUser, getRoles, getUsers, updateRole, updateUser, type ManagedUser, type Role } from '../api/users';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { Tabs, TabPanel } from '../components/ui/Tabs';
import { StatusPill } from '../components/ui/Status';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';

const USER_TABS = ['users', 'roles'] as const;
type UserTab = (typeof USER_TABS)[number];

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
  const [tab, setTab] = useTabParam<UserTab>(USER_TABS);
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

  const tabsId = useTabsId();
  const changeTab = (next: UserTab) => { setTab(next); setSearch(''); };
  const userCreateValid = !!userCreateForm.username && !!userCreateForm.password;

  return (
    <div className="space-y-6">
      <SectionHeader
        title="User management"
        subtitle="Manage roles, permissions, and scoped access for operators."
        action={tab === 'roles' ? <Button icon="add" onClick={openCreateRole}>New role</Button> : <Button icon="person_add" onClick={() => { setUserCreateForm({ username: '', password: '', role: 'viewer', display_name: '', email: '', phone: '' }); setShowUserForm(true); }}>New user</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Users" value={stats.totalUsers} icon="people" />
        <StatCard label="Active" value={stats.enabledUsers} icon="check_circle" />
        <StatCard label="Roles" value={stats.totalRoles} icon="shield" />
        <StatCard label="Custom roles" value={stats.customRoles} icon="tune" />
      </div>

      {users.length > 0 && roles.length > 0 && (
        <Card variant="low" className="p-5">
          <div className="mb-4">
            <h2 className="text-sm font-semibold">Role distribution</h2>
            <p className="text-xs text-fg-muted mt-1">Users by assigned role</p>
          </div>
          <div className="flex h-3 gap-0.5 rounded-full overflow-hidden bg-inset" aria-hidden="true">
            {roles.map((role, index) => {
              const count = users.filter((user) => user.role === role.name).length;
              return count ? <i key={role.id} title={`${role.display_name || role.name}: ${count}`} style={{ width: `${(count / users.length) * 100}%`, background: `var(--color-chart-${(index % 8) + 1})` }} /> : null;
            })}
          </div>
          <ul className="flex flex-wrap gap-x-4 gap-y-2 mt-3">
            {roles.map((role, index) => (
              <li key={role.id} className="inline-flex items-center gap-1.5 text-xs text-fg-muted">
                <i className="inline-block w-2.5 h-2.5 rounded-sm" style={{ background: `var(--color-chart-${(index % 8) + 1})` }} aria-hidden="true" />
                {role.display_name || role.name}
                <span className="font-semibold tabular-nums text-fg">{users.filter((user) => user.role === role.name).length}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <div>
        <Tabs
          label="User management sections"
          idBase={tabsId}
          value={tab}
          onChange={changeTab}
          items={[
            { id: 'users', label: 'Users', icon: 'people', badge: users.length },
            { id: 'roles', label: 'Roles & permissions', icon: 'shield', badge: roles.length },
          ]}
        />
        <TabPanel idBase={tabsId} id={tab} value={tab} className="pt-4">
          <Card variant="low" className="overflow-hidden">
            <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
              <p className="text-sm text-fg-muted">
                <span className="font-semibold tabular-nums text-fg">{tab === 'users' ? filteredUsers.length : filteredRoles.length}</span> {tab === 'users' ? 'users' : 'roles'}
              </p>
              <label className="relative md:w-72">
                <span className="sr-only">Search {tab === 'users' ? 'users' : 'roles'}</span>
                <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
                <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={`Search ${tab === 'users' ? 'users' : 'roles'}`} className={`${INPUT_CLASS} pl-10`} />
              </label>
            </div>

            {loading ? (
              <div className="p-8 text-sm text-fg-muted">Loading…</div>
            ) : tab === 'users' ? (
              filteredUsers.length === 0 ? (
                <EmptyState icon="people" title="No users" description={search ? 'No users match your search.' : 'Users will appear here after registration.'} />
              ) : (
                <ul className="divide-y divide-line">
                  {filteredUsers.map((u) => (
                    <li key={u.id} className="p-4 hover:bg-surface-2 transition-colors">
                      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                        <div className="flex items-center gap-4 min-w-0">
                          <div className="w-10 h-10 rounded-full bg-surface-3 flex items-center justify-center flex-shrink-0 text-sm font-semibold" aria-hidden="true">
                            {(u.display_name || u.username).charAt(0).toUpperCase()}
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <h3 className="font-semibold truncate">{u.display_name || u.username}</h3>
                              {!u.enabled && <StatusPill status="unknown">Disabled</StatusPill>}
                            </div>
                            <p className="text-xs text-fg-muted mt-0.5">@{u.username} · {u.email || 'No email'}</p>
                          </div>
                        </div>
                        <div className="grid grid-cols-[1fr_1fr_auto] gap-4 items-center lg:w-[28rem] flex-shrink-0">
                          <div>
                            <div className="text-xs text-fg-muted">Role</div>
                            <div className="text-sm font-medium capitalize">{u.role?.replace(/_/g, ' ')}</div>
                          </div>
                          <div>
                            <div className="text-xs text-fg-muted">Last login</div>
                            <div className="text-sm font-medium">{timeAgo(u.last_login_at)}</div>
                          </div>
                          <div className="flex items-center gap-1 justify-end">
                            <IconButton icon="edit" label={`Edit ${u.username}`} size="sm" onClick={() => openEditUser(u)} />
                            {u.role !== 'super_admin' && (
                              <IconButton icon="delete" label={`Delete ${u.username}`} size="sm" variant="danger" onClick={() => setDeleteUserTarget(u)} />
                            )}
                          </div>
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )
            ) : filteredRoles.length === 0 ? (
              <EmptyState icon="shield" title="No roles" description="Create a role to define a permission set." action={<Button icon="add" onClick={openCreateRole}>Create role</Button>} />
            ) : (
              <ul className="divide-y divide-line">
                {filteredRoles.map((r) => {
                  const permCount = r.permissions?.length || 0;
                  const groupsHit = new Set((r.permissions || []).map((p) => p.split('.')[0])).size;
                  const roleName = r.display_name || r.name;
                  return (
                    <li key={r.id} className="p-4 hover:bg-surface-2 transition-colors">
                      <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                        <div className="flex items-start gap-4 min-w-0">
                          <div className="w-10 h-10 rounded-md bg-surface-3 flex items-center justify-center flex-shrink-0">
                            <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">shield</span>
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <h3 className="font-semibold truncate">{roleName}</h3>
                              {r.is_system && <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-surface-3 text-fg-muted">System</span>}
                            </div>
                            {r.description && <p className="text-sm text-fg-muted mt-1">{r.description}</p>}
                            <p className="text-xs text-fg-muted mt-2">{permCount} permissions across {groupsHit} groups</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-1 flex-shrink-0">
                          <IconButton icon="edit" label={`Edit role ${roleName}`} size="sm" onClick={() => openEditRole(r)} />
                          {!r.is_system && (
                            <IconButton icon="delete" label={`Delete role ${roleName}`} size="sm" variant="danger" onClick={() => setDeleteTarget(r)} />
                          )}
                        </div>
                      </div>
                      {permCount > 0 && (
                        <div className="flex flex-wrap gap-1 mt-3 pt-3 border-t border-line">
                          {(r.permissions || []).slice(0, 12).map((p) => (
                            <span key={p} className="text-xs font-mono px-2 py-0.5 rounded bg-inset text-fg-muted">{p}</span>
                          ))}
                          {permCount > 12 && <span className="text-xs text-fg-muted px-2 py-0.5">+{permCount - 12} more</span>}
                        </div>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </Card>
        </TabPanel>
      </div>

      <Dialog
        open={!!editingUser}
        onClose={() => setEditingUser(null)}
        title={`Edit user ${editingUser?.username ?? ''}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditingUser(null)}>Cancel</Button>
            <Button type="submit" form="edit-user-form" disabled={submitting}>{submitting ? 'Saving…' : 'Save changes'}</Button>
          </>
        }
      >
        <form id="edit-user-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleUserUpdate(); }}>
          <Field label="Role">
            {(id) => (
              <select id={id} value={asText(userForm.role)} onChange={(e) => setUserForm({ ...userForm, role: e.target.value })} className={INPUT_CLASS}>
                {roles.length > 0
                  ? roles.map((r) => <option key={r.id} value={r.name}>{r.display_name || r.name}</option>)
                  : <option value={asText(userForm.role, 'viewer')}>{asText(userForm.role, 'Viewer')}</option>}
              </select>
            )}
          </Field>
          <Field label="Display name">
            {(id) => <input id={id} value={asText(userForm.display_name)} onChange={(e) => setUserForm({ ...userForm, display_name: e.target.value })} className={INPUT_CLASS} />}
          </Field>
          <Field label="Email">
            {(id) => <input id={id} type="email" value={asText(userForm.email)} onChange={(e) => setUserForm({ ...userForm, email: e.target.value })} className={INPUT_CLASS} />}
          </Field>
          <label className="flex items-center gap-3 text-sm text-fg">
            <input type="checkbox" checked={!!userForm.enabled} onChange={(e) => setUserForm({ ...userForm, enabled: e.target.checked })} className="h-4 w-4 accent-primary" />
            Enabled
          </label>
        </form>
      </Dialog>

      <Dialog
        open={showRoleForm}
        onClose={() => setShowRoleForm(false)}
        size="lg"
        closeOnBackdrop={false}
        title={editingRole ? `Edit role ${editingRole.name}` : 'Create role'}
        description={`${roleForm.permissions.length} permissions selected`}
        headerActions={
          <div className="flex gap-1">
            <Button variant="ghost" size="sm" onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [...ALL_PERMISSIONS] }))}>Select all</Button>
            <Button variant="ghost" size="sm" onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [] }))}>Clear</Button>
          </div>
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowRoleForm(false)}>Cancel</Button>
            <Button type="submit" form="role-form" disabled={submitting || (!editingRole && !roleForm.name)}>{submitting ? 'Saving…' : editingRole ? 'Update role' : 'Create role'}</Button>
          </>
        }
      >
        <form id="role-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleRoleSubmit(); }}>
          {!editingRole && (
            <Field label="Role ID (unique key)">
              {(id) => <input id={id} value={roleForm.name} onChange={(e) => setRoleForm({ ...roleForm, name: e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, '_') })} className={`${INPUT_CLASS} font-mono`} placeholder="network_operator" />}
            </Field>
          )}
          <Field label="Display name">
            {(id) => <input id={id} value={roleForm.display_name} onChange={(e) => setRoleForm({ ...roleForm, display_name: e.target.value })} className={INPUT_CLASS} placeholder="Network operator" />}
          </Field>
          <Field label="Description">
            {(id) => <input id={id} value={roleForm.description} onChange={(e) => setRoleForm({ ...roleForm, description: e.target.value })} className={INPUT_CLASS} placeholder="Can view and acknowledge alerts" />}
          </Field>

          <fieldset className="pt-2">
            <legend className="text-xs font-medium text-fg-muted mb-3">Permissions</legend>
            <div className="space-y-3">
              {PERMISSION_GROUPS.map((group) => {
                const groupPerms = group.permissions.map((p) => p.key);
                const selectedCount = groupPerms.filter((p) => roleForm.permissions.includes(p)).length;
                const allSelected = selectedCount === groupPerms.length;
                const someSelected = selectedCount > 0 && !allSelected;
                return (
                  <div key={group.label} className="bg-inset border border-line rounded-md p-4">
                    <div className="flex items-center gap-3 mb-3">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        ref={(el) => { if (el) el.indeterminate = someSelected; }}
                        onChange={() => toggleGroup(groupPerms)}
                        className="h-4 w-4 accent-primary"
                        id={`grp_${group.label}`}
                      />
                      <span className="material-symbols-outlined text-sm text-fg-muted" aria-hidden="true">{group.icon}</span>
                      <label htmlFor={`grp_${group.label}`} className="font-semibold text-sm cursor-pointer">{group.label}</label>
                      <span className="text-xs text-fg-muted ml-auto tabular-nums">{selectedCount}/{groupPerms.length}</span>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 ml-7">
                      {group.permissions.map((perm) => (
                        <label key={perm.key} className="flex items-start gap-2 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={roleForm.permissions.includes(perm.key)}
                            onChange={() => togglePermission(perm.key)}
                            className="h-4 w-4 accent-primary mt-0.5"
                          />
                          <span>
                            <span className="text-sm text-fg">{perm.label}</span>
                            <span className="block text-xs text-fg-muted">{perm.desc}</span>
                          </span>
                        </label>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </fieldset>
        </form>
      </Dialog>

      <Dialog
        open={showUserForm}
        onClose={() => setShowUserForm(false)}
        closeOnBackdrop={false}
        title="Create user"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowUserForm(false)}>Cancel</Button>
            <Button type="submit" form="create-user-form" disabled={submitting || !userCreateValid}>{submitting ? 'Creating…' : 'Create user'}</Button>
          </>
        }
      >
        <form id="create-user-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleCreateUser(); }}>
          <Field label="Username" required>
            {(id) => <input id={id} required autoComplete="off" value={userCreateForm.username} onChange={(e) => setUserCreateForm({ ...userCreateForm, username: e.target.value })} className={INPUT_CLASS} placeholder="john.doe" />}
          </Field>
          <Field label="Password" required>
            {(id) => <input id={id} required type="password" autoComplete="new-password" value={userCreateForm.password} onChange={(e) => setUserCreateForm({ ...userCreateForm, password: e.target.value })} className={INPUT_CLASS} placeholder="Minimum 6 characters" />}
          </Field>
          <Field label="Role">
            {(id) => (
              <select id={id} value={userCreateForm.role} onChange={(e) => setUserCreateForm({ ...userCreateForm, role: e.target.value })} className={INPUT_CLASS}>
                {roles.length > 0
                  ? roles.map((r) => <option key={r.id} value={r.name}>{r.display_name || r.name}</option>)
                  : ['viewer', 'dept_admin', 'network_admin', 'admin', 'super_admin'].map((r) => <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>)}
              </select>
            )}
          </Field>
          <Field label="Display name">
            {(id) => <input id={id} value={userCreateForm.display_name} onChange={(e) => setUserCreateForm({ ...userCreateForm, display_name: e.target.value })} className={INPUT_CLASS} placeholder="John Doe" />}
          </Field>
          <Field label="Email">
            {(id) => <input id={id} type="email" value={userCreateForm.email} onChange={(e) => setUserCreateForm({ ...userCreateForm, email: e.target.value })} className={INPUT_CLASS} placeholder="john@example.com" />}
          </Field>
          <Field label="Phone">
            {(id) => <input id={id} type="tel" value={userCreateForm.phone} onChange={(e) => setUserCreateForm({ ...userCreateForm, phone: e.target.value })} className={INPUT_CLASS} placeholder="+1 555 123 4567" />}
          </Field>
        </form>
      </Dialog>

      <ConfirmDialog open={!!deleteUserTarget} title="Delete user?" message={`Delete user "${deleteUserTarget?.username}"? This action cannot be undone.`} confirmLabel="Delete user" danger onConfirm={handleDeleteUser} onCancel={() => setDeleteUserTarget(null)} />

      <ConfirmDialog open={!!deleteTarget} title="Delete role?" message={`Delete "${deleteTarget?.display_name || deleteTarget?.name}"? Users with this role will lose its permissions.`} confirmLabel="Delete role" danger onConfirm={handleDeleteRole} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';

function Field({ label, required, children }: { label: string; required?: boolean; children: (id: string) => React.ReactNode }) {
  const id = useId();
  return (
    <div>
      <label htmlFor={id} className="text-xs font-medium text-fg-muted block mb-1.5">
        {label}{required && <span className="text-error" aria-hidden="true"> *</span>}
      </label>
      {children(id)}
    </div>
  );
}
