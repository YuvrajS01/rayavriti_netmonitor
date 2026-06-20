import { useState, useEffect, useCallback, useMemo } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
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

const ROLES = ['super_admin', 'admin', 'operator', 'viewer'] as const;

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
  const [submitting, setSubmitting] = useState(false);

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
    systemRoles: roles.filter((r) => r.is_system).length,
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

  return (
    <div className="space-y-8">
      <SectionHeader
        title="User Management"
        subtitle="Manage roles, permissions, and scoped access for operators."
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Users" value={stats.totalUsers} icon="people" />
        <StatCard label="Active" value={stats.enabledUsers} icon="check_circle" />
        <StatCard label="Roles" value={stats.totalRoles} icon="shield" />
        <StatCard label="System Roles" value={stats.systemRoles} icon="admin_panel_settings" />
      </div>

      <div className="flex gap-1 bg-surface-container-low rounded-lg p-1 w-fit">
        {(['users', 'roles'] as const).map((t) => (
          <button key={t} onClick={() => { setTab(t); setSearch(''); }} className={`px-4 py-2 text-xs font-headline font-bold uppercase tracking-wide rounded-md transition-colors ${tab === t ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:text-on-surface'}`}>
            {t === 'users' ? 'Users' : 'Roles'}
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
              <h2 className="font-headline font-bold text-lg">{tab === 'users' ? 'Users' : 'Roles'}</h2>
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
          <EmptyState icon="shield" title="No roles" description="Roles define permission sets for users." />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filteredRoles.map((r) => (
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
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-1.5 lg:max-w-xl">
                    {(r.permissions || []).slice(0, 8).map((p) => (
                      <span key={p} className="text-[10px] font-mono px-2 py-0.5 rounded bg-surface-container-highest text-on-surface-variant">{p}</span>
                    ))}
                    {(r.permissions || []).length > 8 && <span className="text-[10px] text-on-surface-variant px-2 py-0.5">+{(r.permissions || []).length - 8} more</span>}
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {editingUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60" onClick={() => setEditingUser(null)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg overflow-hidden" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20"><h2 className="font-headline text-lg font-bold">Edit User: {editingUser.username}</h2></div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Role</label>
                <select value={String(userForm.role ?? '')} onChange={(e) => setUserForm({ ...userForm, role: e.target.value })} className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary">
                  {ROLES.map((r) => <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>)}
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
    </div>
  );
}
