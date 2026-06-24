import { useState, useEffect, useMemo } from 'react';
import { listPhase2, createPhase2, updatePhase2, type Phase2Row } from '../api/phase2';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface Contact extends Phase2Row {
  id: number;
  name: string;
  designation: string;
  department: string;
  email: string;
  phone: string;
  telegram_chat_id: string;
  whatsapp_number: string;
  preferred_channel: string;
  notification_enabled: boolean;
  quiet_hours_start: string;
  quiet_hours_end: string;
  enabled: boolean;
}

const CHANNELS = ['email', 'telegram', 'whatsapp', 'sms'] as const;

function channelIcon(ch: string) {
  if (ch === 'email') return 'mail';
  if (ch === 'telegram') return 'send';
  if (ch === 'whatsapp') return 'chat';
  if (ch === 'sms') return 'sms';
  return 'notifications';
}

const EMPTY_CONTACT = {
  name: '', designation: '', department: '', email: '', phone: '',
  preferred_channel: 'email', notification_enabled: true, enabled: true,
};

export default function Contacts() {
  const { addToast } = useToast();
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [channelFilter, setChannelFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Contact | null>(null);
  const [form, setForm] = useState<Record<string, unknown>>(EMPTY_CONTACT);
  const [submitting, setSubmitting] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Contact | null>(null);

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const res = await listPhase2('/contacts');
        if (active) setContacts((res.data || []) as Contact[]);
      } catch {
        if (active) setContacts([]);
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, []);

  const load = async () => {
    const res = await listPhase2('/contacts');
    setContacts((res.data || []) as Contact[]);
  };

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return contacts.filter((c) => {
      const matchSearch = !needle || `${c.name} ${c.designation} ${c.department} ${c.email}`.toLowerCase().includes(needle);
      const matchChannel = channelFilter === 'all' || c.preferred_channel === channelFilter;
      return matchSearch && matchChannel;
    });
  }, [contacts, search, channelFilter]);

  const stats = useMemo(() => ({
    total: contacts.length,
    enabled: contacts.filter((c) => c.enabled).length,
    telegram: contacts.filter((c) => c.preferred_channel === 'telegram').length,
    email: contacts.filter((c) => c.preferred_channel === 'email').length,
  }), [contacts]);

  const openCreate = () => { setEditing(null); setForm(EMPTY_CONTACT); setShowForm(true); };
  const openEdit = (c: Contact) => { setEditing(c); setForm({ ...c }); setShowForm(true); };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      if (editing) {
        await updatePhase2('/contacts', editing.id, form);
        addToast('Contact updated', 'success');
      } else {
        await createPhase2('/contacts', form);
        addToast('Contact created', 'success');
      }
      setShowForm(false);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Save failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      const { v1 } = await import('../api/http');
      await v1.delete(`/contacts/${deleteTarget.id}`);
      addToast('Contact deleted', 'success');
      setDeleteTarget(null);
      await load();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title="Contact Directory"
        subtitle="Manage escalation contacts, notification channels, and quiet hours."
        action={<Button icon="add" onClick={openCreate}>New Contact</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="Total" value={stats.total} icon="contacts" />
        <StatCard label="Enabled" value={stats.enabled} icon="check_circle" />
        <StatCard label="Telegram" value={stats.telegram} icon="send" />
        <StatCard label="Email" value={stats.email} icon="mail" />
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">contacts</span>
            </div>
            <div>
              <h2 className="font-headline font-bold text-lg">Contacts</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{filtered.length} contacts</p>
            </div>
          </div>
          <div className="flex gap-3">
            <select
              value={channelFilter}
              onChange={(e) => setChannelFilter(e.target.value)}
              className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2.5 text-xs text-on-surface outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="all">All channels</option>
              {CHANNELS.map((ch) => <option key={ch} value={ch}>{ch.charAt(0).toUpperCase() + ch.slice(1)}</option>)}
            </select>
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search contacts..."
              className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-64"
            />
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="contacts" title="No contacts found" description="Add contacts for escalation and notification routing." action={<Button icon="add" onClick={openCreate}>Add Contact</Button>} />
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filtered.map((c) => (
              <article key={c.id} className="p-5 hover:bg-surface-container-high/50 transition-colors">
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className="w-10 h-10 rounded-full bg-surface-container-highest flex items-center justify-center flex-shrink-0">
                      <span className="material-symbols-outlined text-primary text-xl">{channelIcon(c.preferred_channel)}</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-headline font-bold text-lg truncate">{c.name}</h3>
                      <p className="text-xs text-on-surface-variant mt-0.5">
                        {c.designation || 'No designation'} {c.department ? `· ${c.department}` : ''}
                      </p>
                      <div className="flex flex-wrap gap-3 mt-2">
                        {c.email && (
                          <span className="text-xs text-on-surface-variant flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm">mail</span>{c.email}
                          </span>
                        )}
                        {c.phone && (
                          <span className="text-xs text-on-surface-variant flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm">phone</span>{c.phone}
                          </span>
                        )}
                        {c.telegram_chat_id && (
                          <span className="text-xs text-on-surface-variant flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm">send</span>Telegram
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 lg:max-w-xl flex-shrink-0">
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Channel</div>
                      <div className="text-sm font-medium capitalize">{c.preferred_channel || '-'}</div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Quiet Hours</div>
                      <div className="text-sm font-medium">{c.quiet_hours_start && c.quiet_hours_end ? `${c.quiet_hours_start}–${c.quiet_hours_end}` : 'None'}</div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Notifications</div>
                      <div className={`text-sm font-medium ${c.notification_enabled ? 'text-success' : 'text-outline'}`}>
                        {c.notification_enabled ? 'Enabled' : 'Disabled'}
                      </div>
                    </div>
                    <div>
                      <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">Status</div>
                      <div className={`text-sm font-medium ${c.enabled ? 'text-success' : 'text-outline'}`}>
                        {c.enabled ? 'Active' : 'Inactive'}
                      </div>
                    </div>
                  </div>
                </div>
                <div className="flex gap-2 mt-3 pt-3 border-t border-outline-variant/10">
                  <button onClick={() => openEdit(c)} className="text-xs font-bold text-primary hover:bg-primary/10 px-3 py-1 rounded transition-colors">Edit</button>
                  <button onClick={() => setDeleteTarget(c)} className="text-xs font-bold text-error hover:bg-error/10 px-3 py-1 rounded transition-colors">Delete</button>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>

      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={() => setShowForm(false)}>
          <div className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-lg max-h-[90vh] overflow-hidden flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="p-6 border-b border-outline-variant/20 shrink-0">
              <h2 className="font-headline text-lg font-bold">{editing ? 'Edit Contact' : 'New Contact'}</h2>
            </div>
            <div className="p-6 space-y-4 flex-1 min-h-0 overflow-y-auto">
              {(['name', 'designation', 'department', 'email', 'phone'] as const).map((f) => (
                <div key={f}>
                  <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">{f.replace(/_/g, ' ')}</label>
                  <input
                    value={String(form[f] ?? '')}
                    onChange={(e) => setForm({ ...form, [f]: e.target.value })}
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
                  />
                </div>
              ))}
              <div>
                <label className="text-[10px] text-on-surface-variant uppercase tracking-wide block mb-1">Preferred Channel</label>
                <select
                  value={String(form.preferred_channel ?? 'email')}
                  onChange={(e) => setForm({ ...form, preferred_channel: e.target.value })}
                  className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary"
                >
                  {CHANNELS.map((ch) => <option key={ch} value={ch}>{ch.charAt(0).toUpperCase() + ch.slice(1)}</option>)}
                </select>
              </div>
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={!!form.notification_enabled}
                  onChange={(e) => setForm({ ...form, notification_enabled: e.target.checked })}
                  className="accent-primary"
                  id="notif_enabled"
                />
                <label htmlFor="notif_enabled" className="text-sm text-on-surface">Notifications enabled</label>
              </div>
              <div className="flex items-center gap-3">
                <input
                  type="checkbox"
                  checked={!!form.enabled}
                  onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                  className="accent-primary"
                  id="contact_enabled"
                />
                <label htmlFor="contact_enabled" className="text-sm text-on-surface">Active</label>
              </div>
            </div>
            <div className="flex border-t border-outline-variant/20 shrink-0">
              <button onClick={() => setShowForm(false)} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-on-surface-variant hover:bg-surface-container-high transition-colors">Cancel</button>
              <div className="w-px bg-outline-variant/20" />
              <button onClick={handleSubmit} disabled={submitting} className="flex-1 py-3 text-xs font-headline font-bold uppercase tracking-wide text-primary hover:bg-primary/10 transition-colors disabled:opacity-50">
                {submitting ? 'Saving...' : editing ? 'Update' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Contact"
        message={`Delete "${deleteTarget?.name}"? This removes them from all escalation paths.`}
        confirmLabel="Delete"
        danger
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
