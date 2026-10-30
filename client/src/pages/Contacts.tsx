import { useState, useMemo } from 'react';
import type { ResourceRow } from '../api/resources';
import { useResourceList, useResourceMutations } from '../hooks/useResources';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import StatCard from '../components/ui/StatCard';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import Dialog from '../components/ui/Dialog';
import IconButton from '../components/ui/IconButton';
import { StatusPill } from '../components/ui/Status';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';

interface Contact extends ResourceRow {
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

const EMPTY_CONTACTS: Contact[] = [];

export default function Contacts() {
  const { addToast } = useToast();
  const contactsQuery = useResourceList<Contact>('/contacts');
  const contacts = contactsQuery.data ?? EMPTY_CONTACTS;
  const loading = contactsQuery.isPending;
  const [search, setSearch] = useState('');
  const [channelFilter, setChannelFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Contact | null>(null);
  const [form, setForm] = useState<Record<string, unknown>>(EMPTY_CONTACT);
  const [deleteTarget, setDeleteTarget] = useState<Contact | null>(null);

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

  const { save, remove } = useResourceMutations('/contacts');
  const submitting = save.isPending;

  const handleSubmit = () => {
    const updating = editing != null;
    save.mutate({ id: editing?.id, body: form }, {
      onSuccess: () => {
        addToast(updating ? 'Contact updated' : 'Contact created', 'success');
        setShowForm(false);
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Save failed', 'error'),
    });
  };

  const handleDelete = () => {
    if (!deleteTarget) return;
    remove.mutate(deleteTarget.id, {
      onSuccess: () => {
        addToast('Contact deleted', 'success');
        setDeleteTarget(null);
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
    });
  };

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Contacts"
        subtitle="Manage escalation contacts, notification channels, and quiet hours."
        action={<Button icon="add" onClick={openCreate}>New contact</Button>}
      />

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total" value={stats.total} icon="contacts" />
        <StatCard label="Enabled" value={stats.enabled} icon="check_circle" />
        <StatCard label="Telegram" value={stats.telegram} icon="send" />
        <StatCard label="Email" value={stats.email} icon="mail" />
      </div>

      <Card variant="low" className="overflow-hidden">
        <div className="p-4 border-b border-line flex flex-col md:flex-row gap-3 md:items-center md:justify-between">
          <p className="text-sm text-fg-muted"><span className="font-semibold tabular-nums text-fg">{filtered.length}</span> contacts</p>
          <div className="flex gap-3">
            <label>
              <span className="sr-only">Filter by channel</span>
              <select value={channelFilter} onChange={(e) => setChannelFilter(e.target.value)} className={INPUT_CLASS}>
                <option value="all">All channels</option>
                {CHANNELS.map((ch) => <option key={ch} value={ch}>{ch.charAt(0).toUpperCase() + ch.slice(1)}</option>)}
              </select>
            </label>
            <label className="relative md:w-64">
              <span className="sr-only">Search contacts</span>
              <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
              <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search contacts" className={`${INPUT_CLASS} pl-10`} />
            </label>
          </div>
        </div>

        {loading ? (
          <div className="p-8 text-sm text-fg-muted">Loading…</div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="contacts" title="No contacts found" description="Add contacts for escalation and notification routing." action={<Button icon="add" onClick={openCreate}>Add contact</Button>} />
        ) : (
          <ul className="divide-y divide-line">
            {filtered.map((c) => (
              <li key={c.id} className="p-4 hover:bg-surface-2 transition-colors">
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                  <div className="flex items-start gap-4 min-w-0">
                    <div className="w-10 h-10 rounded-full bg-surface-3 flex items-center justify-center flex-shrink-0">
                      <span className="material-symbols-outlined text-fg-muted text-xl" aria-hidden="true">{channelIcon(c.preferred_channel)}</span>
                    </div>
                    <div className="min-w-0">
                      <h3 className="font-semibold truncate">{c.name}</h3>
                      <p className="text-xs text-fg-muted mt-0.5">
                        {c.designation || 'No designation'} {c.department ? `· ${c.department}` : ''}
                      </p>
                      <div className="flex flex-wrap gap-3 mt-2">
                        {c.email && (
                          <span className="text-xs text-fg-muted flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm" aria-hidden="true">mail</span>{c.email}
                          </span>
                        )}
                        {c.phone && (
                          <span className="text-xs text-fg-muted flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm" aria-hidden="true">phone</span>{c.phone}
                          </span>
                        )}
                        {c.telegram_chat_id && (
                          <span className="text-xs text-fg-muted flex items-center gap-1">
                            <span className="material-symbols-outlined text-sm" aria-hidden="true">send</span>Telegram
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-start gap-4 flex-shrink-0">
                    <dl className="grid grid-cols-2 sm:grid-cols-4 gap-4 lg:max-w-xl">
                      <div>
                        <dt className="text-xs text-fg-muted">Channel</dt>
                        <dd className="text-sm font-medium capitalize">{c.preferred_channel || '-'}</dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Quiet hours</dt>
                        <dd className="text-sm font-medium tabular-nums">{c.quiet_hours_start && c.quiet_hours_end ? `${c.quiet_hours_start}–${c.quiet_hours_end}` : 'None'}</dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Notifications</dt>
                        <dd className="mt-0.5"><StatusPill status={c.notification_enabled ? 'up' : 'unknown'}>{c.notification_enabled ? 'On' : 'Off'}</StatusPill></dd>
                      </div>
                      <div>
                        <dt className="text-xs text-fg-muted">Status</dt>
                        <dd className="mt-0.5"><StatusPill status={c.enabled ? 'up' : 'unknown'}>{c.enabled ? 'Active' : 'Inactive'}</StatusPill></dd>
                      </div>
                    </dl>
                    <div className="flex gap-1">
                      <IconButton icon="edit" label={`Edit ${c.name}`} size="sm" onClick={() => openEdit(c)} />
                      <IconButton icon="delete" label={`Delete ${c.name}`} size="sm" variant="danger" onClick={() => setDeleteTarget(c)} />
                    </div>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Dialog
        open={showForm}
        onClose={() => setShowForm(false)}
        closeOnBackdrop={false}
        title={editing ? 'Edit contact' : 'New contact'}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button type="submit" form="contact-form" disabled={submitting}>{submitting ? 'Saving…' : editing ? 'Save changes' : 'Create contact'}</Button>
          </>
        }
      >
        <form id="contact-form" className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
          {(['name', 'designation', 'department', 'email', 'phone'] as const).map((f) => (
            <div key={f}>
              <label htmlFor={`contact_${f}`} className="text-xs font-medium text-fg-muted block mb-1.5">{f.charAt(0).toUpperCase() + f.slice(1).replace(/_/g, ' ')}</label>
              <input
                id={`contact_${f}`}
                type={f === 'email' ? 'email' : f === 'phone' ? 'tel' : 'text'}
                value={asText(form[f])}
                onChange={(e) => setForm({ ...form, [f]: e.target.value })}
                className={INPUT_CLASS}
              />
            </div>
          ))}
          <div>
            <label htmlFor="contact_channel" className="text-xs font-medium text-fg-muted block mb-1.5">Preferred channel</label>
            <select id="contact_channel" value={asText(form.preferred_channel, 'email')} onChange={(e) => setForm({ ...form, preferred_channel: e.target.value })} className={INPUT_CLASS}>
              {CHANNELS.map((ch) => <option key={ch} value={ch}>{ch.charAt(0).toUpperCase() + ch.slice(1)}</option>)}
            </select>
          </div>
          <label className="flex items-center gap-3 text-sm text-fg">
            <input type="checkbox" checked={!!form.notification_enabled} onChange={(e) => setForm({ ...form, notification_enabled: e.target.checked })} className="h-4 w-4 accent-primary" />
            Notifications enabled
          </label>
          <label className="flex items-center gap-3 text-sm text-fg">
            <input type="checkbox" checked={!!form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} className="h-4 w-4 accent-primary" />
            Active
          </label>
        </form>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete contact?"
        message={`Delete "${deleteTarget?.name}"? This removes them from all escalation paths.`}
        confirmLabel="Delete contact"
        danger
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}

const INPUT_CLASS = 'w-full h-control bg-inset border border-line rounded-md px-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary';
