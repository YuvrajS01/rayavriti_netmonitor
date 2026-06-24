import { useState, useEffect, useCallback, useMemo } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface User {
  id: number;
  username: string;
  role: string;
  display_name: string;
  email: string;
  phone: string;
  enabled: boolean;
  last_login_at: string | null;
  created_at: string;
  role_id: number;
}

interface Role {
  id: number;
  name: string;
  display_name: string;
  description: string;
  permissions: string[];
  is_system: boolean;
}

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
    label: 'Contacts',
    icon: 'contacts',
    permissions: [
      { key: 'contacts.write', label: 'Manage contacts', desc: 'Add and edit escalation contacts' },
    ],
  },
  {
    label: 'Reports',
    icon: 'analytics',
    permissions: [
      { key: 'reports.read', label: 'View reports', desc: 'Access reports and dashboards' },
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
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [tab, setTab] = useState<'users' | 'roles'>('users');
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [userForm, setUserForm] = useState<Record<string, unknown>>({});
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [showRoleForm, setShowRoleForm] = useState(false);
  const [roleForm, setRoleForm] = useState({ name: '', display_name: '', description: '', permissions: [] as string[] });
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    const [usersRes, rolesRes] = await Promise.all([
      v1.get('/users'),
      v1.get('/roles'),
    ]);
    setUsers(wrap<User[]>(usersRes.data).data || []);
    setRoles(wrap<Role[]>(rolesRes.data).data || []);
    setLoading(false);
  }, []);

  useEffect(() => { load().catch(() => setLoading(false)); }, [load]);

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

  const openEditUser = (u: User) => {
    setEditingUser(u);
    setUserForm({ role: u.role, display_name: u.display_name, email: u.email, phone: u.phone, enabled: u.enabled });
  };

  const handleUserUpdate = async () => {
    if (!editingUser) return;
    setSubmitting(true);
    try {
      await v1.put(`/users/${editingUser.id}`, userForm);
      addToast('User updated', 'success');
      setEditingUser(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Update failed', 'error');
    } finally {
      setSubmitting(false);
    }
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

  const handleRoleSubmit = async () => {
    setSubmitting(true);
    try {
      if (editingRole) {
        await v1.put(`/roles/${editingRole.id}`, {
          display_name: roleForm.display_name,
          description: roleForm.description,
          permissions: roleForm.permissions,
        });
        addToast('Role updated', 'success');
      } else {
        await v1.post('/roles', {
          name: roleForm.name,
          display_name: roleForm.display_name,
          description: roleForm.description,
          permissions: roleForm.permissions,
          is_system: false,
        });
        addToast('Role created', 'success');
      }
      setShowRoleForm(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Save failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteRole = async () => {
    if (!deleteTarget) return;
    try {
      await v1.delete(`/roles/${deleteTarget.id}`);
      addToast('Role deleted', 'success');
      setDeleteTarget(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="User Management"
        subtitle="Manage roles, permissions, and scoped access for operators."
        action={tab === 'roles' ? <Button icon="add" onClick={openCreateRole}>New Role</Button> : undefined}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Users" value={stats.totalUsers} icon="people" />
        <StatCard label="Active" value={stats.enabledUsers} icon="check_circle" />
        <StatCard label="Roles" value={stats.totalRoles} icon="shield" />
        <StatCard label="Custom Roles" value={stats.customRoles} icon="tune" />
      </div>

      <div className="flex gap-1 bg-surface-container-low rounded-lg p-1 w-fit">
        {(['users', 'roles'] as const).map((t) => (
          <button key={t} onClick={() => { setTab(t); setSearch(''); }} className={`px-4 py-2 text-xs font-headline font-bold uppercase tracking-wide rounded-md transition-colors ${tab === t ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:text-on-surface'}`}>
            {t === 'users' ? 'Users' : 'Roles & Permissions'}
          </button>
        ))}
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">{tab === 'users' ? 'people' : 'shield'}</span>
            </div>
            <div>
              <h2 className="font-headline font-bold text-lg">{tab === 'users' ? 'Users' : 'Roles & Permissions'}</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{tab === 'users' ? filteredUsers.length : filteredRoles.length} items</p>
            </div>
          </div>
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search..." className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-64" />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : tab === 'users' ? (
          filteredUsers.length === 0 ? (
            <EmptyState icon="people" title="No users" description="Users will appear here after registration." />
          ) : (
            <div className="divide-y divide-outline-variant/20">
              {filteredUsers.map((u) => (
                <article key={u.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
                    <div className="flex items-center gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-full bg-surface-container-highest flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">person</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-headline font-bold text-lg truncate">{u.display_name || u.username}</h3>
                          {!u.enabled && <span className="text-[10px] font-bold uppercase px-2 py-0.5 rounded-full bg-outline/10 text-outline">Disabled</span>}
                        </div>
                        <p className="text-xs text-on-surface-variant mt-0.5">@{u.username} · {u.email || 'No email'}</p>
                      </div>
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 lg:max-w-lg flex-shrink-0">
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Role</div>
                        <div className="text-sm font-bold capitalize text-primary">{u.role?.replace(/_/g, ' ')}</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Last Login</div>
                        <div className="text-sm font-medium">{timeAgo(u.last_login_at)}</div>
                      </div>
                      <div className="flex items-end gap-2">
                        <button onClick={() => openEditUser(u)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
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
          <div className="divide-y divide-outline-variant/20">
            {filteredRoles.map((r) => {
              const permCount = r.permissions?.length || 0;
              const groupsHit = new Set((r.permissions || []).map((p) => p.split('.')[0])).size;
              return (
                <article key={r.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                  <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                    <div className="flex items-start gap-4 min-w-0">
                      <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0">
                        <span className="material-symbols-outlined text-primary text-xl">shield</span>
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h3 className="font-headline font-bold text-lg truncate">{r.display_name || r.name}</h3>
                          {r.is_system && <span className="text-[10px] font-bold uppercase px-2 py-0.5 rounded-full bg-primary/10 text-primary">System</span>}
                        </div>
                        {r.description && <p className="text-sm text-on-surface-variant mt-1">{r.description}</p>}
                        <div className="flex items-center gap-3 mt-2">
                          <span className="text-xs text-on-surface-variant">{permCount} permissions across {groupsHit} groups</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <button onClick={() => openEditRole(r)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1.5 rounded transition-colors">Edit</button>
                      {!r.is_system && (
                        <button onClick={() => setDeleteTarget(r)} className="text-xs font-bold text-error hover:bg-error/10 px-3 py-1.5 rounded transition-colors">Delete</button>
                      )}
                    </div>
                  </div>
                  {permCount > 0 && (
                    <div className="flex flex-wrap gap-1 mt-3 pt-3 border-t border-outline-variant/10">
                      {(r.permissions || []).slice(0, 12).map((p) => (
                        <span key={p} className="text-[10px] font-mono px-2 py-0.5 rounded bg-surface-container-highest text-on-surface-variant">{p}</span>
                      ))}
                      {permCount > 12 && <span className="text-[10px] text-on-surface-variant px-2 py-0.5">+{permCount - 12} more</span>}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        )}
      </Card>

      {editingUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60" onClick={() => setEditingUser(null)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20"><h2 className="font-headline text-lg font-bold">Edit User: {editingUser.username}</h2></div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Role</label>
                <select value={String(userForm.role ?? '')} onChange={(e) => setUserForm({ ...userForm, role: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                  {roles.map((r) => <option key={r.id} value={r.name}>{r.display_name || r.name}</option>)}
                </select>
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Display Name</label>
                <input value={String(userForm.display_name ?? '')} onChange={(e) => setUserForm({ ...userForm, display_name: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Email</label>
                <input value={String(userForm.email ?? '')} onChange={(e) => setUserForm({ ...userForm, email: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" />
              </div>
              <div className="flex items-center gap-3">
                <input type="checkbox" checked={!!userForm.enabled} onChange={(e) => setUserForm({ ...userForm, enabled: e.target.checked })} className="accent-primary" id="user_enabled" />
                <label htmlFor="user_enabled" className="text-sm text-on-surface">Enabled</label>
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20">
              <button onClick={() => setEditingUser(null)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleUserUpdate} disabled={submitting} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : 'Update'}</button>
            </div>
          </div>
        </div>
      )}

      {showRoleForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60" onClick={() => setShowRoleForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-2xl max-h-[90vh] overflow-hidden" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 flex items-center justify-between">
              <div>
                <h2 className="font-headline text-lg font-bold">{editingRole ? `Edit Role: ${editingRole.name}` : 'Create Role'}</h2>
                <p className="text-xs text-on-surface-variant mt-1">{roleForm.permissions.length} permissions selected</p>
              </div>
              <div className="flex gap-2">
                <button onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [...ALL_PERMISSIONS] }))} className="text-[10px] font-bold uppercase tracking-wide text-primary hover:bg-primary/10 px-3 py-1.5 rounded transition-colors">Select All</button>
                <button onClick={() => setRoleForm((prev) => ({ ...prev, permissions: [] }))} className="text-[10px] font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high px-3 py-1.5 rounded transition-colors">Clear</button>
              </div>
            </div>
            <div className="p-6 space-y-4 max-h-[55vh] overflow-y-auto">
              {!editingRole && (
                <div>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Role ID (unique key)</label>
                  <input value={roleForm.name} onChange={(e) => setRoleForm({ ...roleForm, name: e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, '_') })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface font-mono placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="network_operator" />
                </div>
              )}
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Display Name</label>
                <input value={roleForm.display_name} onChange={(e) => setRoleForm({ ...roleForm, display_name: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="Network Operator" />
              </div>
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Description</label>
                <input value={roleForm.description} onChange={(e) => setRoleForm({ ...roleForm, description: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none" placeholder="Can view and acknowledge alerts" />
              </div>

              <div className="pt-2">
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-3">Permissions</label>
                <div className="space-y-4">
                  {PERMISSION_GROUPS.map((group) => {
                    const groupPerms = group.permissions.map((p) => p.key);
                    const selectedCount = groupPerms.filter((p) => roleForm.permissions.includes(p)).length;
                    const allSelected = selectedCount === groupPerms.length;
                    const someSelected = selectedCount > 0 && !allSelected;
                    return (
                      <div key={group.label} className="bg-surface-container-highest rounded-lg p-4">
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
                          <label htmlFor={`grp_${group.label}`} className="font-headline font-bold text-sm cursor-pointer">{group.label}</label>
                          <span className="text-[10px] text-on-surface-variant ml-auto">{selectedCount}/{groupPerms.length}</span>
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
                                <span className="text-sm text-on-surface group-hover:text-primary transition-colors">{perm.label}</span>
                                <span className="block text-[11px] text-on-surface-variant">{perm.desc}</span>
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
            <div className="flex border-t border-outline-variant/20">
              <button onClick={() => setShowRoleForm(false)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleRoleSubmit} disabled={submitting || (!editingRole && !roleForm.name)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">{submitting ? 'Saving...' : editingRole ? 'Update Role' : 'Create Role'}</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog open={!!deleteTarget} title="Delete Role" message={`Delete "${deleteTarget?.display_name || deleteTarget?.name}"? Users with this role will lose its permissions.`} confirmLabel="Delete" danger onConfirm={handleDeleteRole} onCancel={() => setDeleteTarget(null)} />
    </div>
  );
}
