import { useId, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { addAlertNote, getAlertNotes, getAssignableUsers, setAlertAssignee } from '../api/alerts';
import { patchAlertLists } from '../api/liveCache';
import { queryKeys } from '../api/queryKeys';
import type { Alert, AlertNote } from '../api/types';
import Dialog from './ui/Dialog';
import Button from './ui/Button';
import RelativeTime from './ui/RelativeTime';
import { StatusPill } from './ui/Status';
import { useToast } from './ui/useToast';
import { lifecycleMeta, severityMeta } from '../utils/status';
import { formatAbsolute } from '../utils/time';

interface AlertDrawerProps {
  alert: Alert | null;
  onClose: () => void;
  canAssign: boolean;
  canNote: boolean;
  /** Focus the note box on open (the `n` shortcut). */
  focusNote?: boolean;
}

const NOTE_MAX = 2000;

/** Side panel for one alert: summary, assignee and the notes thread. */
export default function AlertDrawer({ alert, onClose, canAssign, canNote, focusNote = false }: AlertDrawerProps) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [text, setText] = useState('');
  const noteRef = useRef<HTMLTextAreaElement>(null);
  const assigneeId = useId();
  const id = alert?.id ?? 0;

  const notes = useQuery({
    queryKey: queryKeys.alerts.notes(id),
    queryFn: ({ signal }) => getAlertNotes(id, { signal }).then((r) => r.data ?? []),
    enabled: alert != null,
    retry: false,
  });
  const users = useQuery({
    queryKey: queryKeys.alerts.assignable(),
    queryFn: ({ signal }) => getAssignableUsers({ signal }).then((r) => r.data ?? []),
    enabled: alert != null && canAssign,
    retry: false,
    staleTime: 5 * 60_000,
  });

  const addNote = useMutation({
    mutationFn: (body: string) => addAlertNote(id, body),
    onSuccess: (res) => {
      queryClient.setQueryData<AlertNote[]>(queryKeys.alerts.notes(id), (prev) => [...(prev ?? []), res.data]);
      setText('');
    },
    onError: () => addToast('Could not add the note', 'error'),
  });

  const assign = useMutation({
    mutationFn: (userId: number | null) => setAlertAssignee(id, userId),
    onMutate: (userId) => {
      const name = userId == null ? null : users.data?.find((u) => u.id === userId)?.displayName ?? null;
      patchAlertLists(queryClient, { id, assignedTo: userId, assignedToName: name });
    },
    onSuccess: (res) => { if (res.data?.id) patchAlertLists(queryClient, res.data); },
    onError: () => {
      addToast('Could not change the assignee', 'error');
      void queryClient.invalidateQueries({ queryKey: [...queryKeys.alerts.all, 'list'] });
    },
  });

  if (!alert) return null;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const body = text.trim();
    if (body && body.length <= NOTE_MAX) addNote.mutate(body);
  };

  return (
    <Dialog
      open
      onClose={onClose}
      placement="drawer"
      title={alert.message}
      description={<>{alert.deviceName || `Device ${alert.deviceId}`} · <span title={formatAbsolute(alert.createdAt)}><RelativeTime value={alert.createdAt} /></span></>}
      initialFocusRef={focusNote ? noteRef : undefined}
    >
      <div className="space-y-6">
        <div className="flex flex-wrap items-center gap-2">
          <StatusPill status={severityMeta(alert.severity)} />
          <StatusPill status={lifecycleMeta(alert.status)} />
          <Link to={`/devices/${alert.deviceId}`} className="ml-auto text-sm text-primary underline">Open device</Link>
        </div>
        <dl className="grid grid-cols-2 gap-3 text-sm">
          <div><dt className="text-xs text-fg-muted">Raised</dt><dd>{formatAbsolute(alert.createdAt)}</dd></div>
          {alert.acknowledgedBy && <div><dt className="text-xs text-fg-muted">Acknowledged by</dt><dd>{alert.acknowledgedBy}</dd></div>}
          {alert.resolvedAt && <div><dt className="text-xs text-fg-muted">Resolved</dt><dd>{formatAbsolute(alert.resolvedAt)}</dd></div>}
        </dl>

        <section aria-labelledby={`${assigneeId}-h`}>
          <h3 id={`${assigneeId}-h`} className="mb-2 text-sm font-semibold">Assignee</h3>
          {!canAssign ? (
            <p className="text-sm text-fg-muted">{alert.assignedToName || 'Unassigned'}</p>
          ) : users.isError ? (
            <p className="text-sm text-fg-muted">Assignment isn't available on this server. Currently: {alert.assignedToName || 'unassigned'}.</p>
          ) : (
            <>
              <label htmlFor={assigneeId} className="sr-only">Assign to</label>
              <select
                id={assigneeId}
                value={alert.assignedTo != null ? String(alert.assignedTo) : ''}
                disabled={users.isPending || assign.isPending}
                onChange={(e) => assign.mutate(e.target.value ? Number(e.target.value) : null)}
                className="h-control w-full rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary"
              >
                <option value="">Unassigned</option>
                {(users.data ?? []).map((u) => <option key={u.id} value={String(u.id)}>{u.displayName || u.username}</option>)}
              </select>
            </>
          )}
        </section>

        <section aria-labelledby={`${assigneeId}-notes`}>
          <h3 id={`${assigneeId}-notes`} className="mb-2 text-sm font-semibold">Notes</h3>
          {notes.isPending ? (
            <p role="status" className="text-sm text-fg-muted">Loading notes…</p>
          ) : notes.isError ? (
            <p className="text-sm text-fg-muted">Notes aren't available on this server.</p>
          ) : notes.data.length === 0 ? (
            <p className="text-sm text-fg-muted">No notes yet.</p>
          ) : (
            <ol className="space-y-3">
              {notes.data.map((n) => (
                <li key={n.id} className="rounded-md border border-line bg-surface-2 p-3">
                  <p className="mb-1 text-xs text-fg-muted"><span className="font-medium text-fg">{n.author}</span> · <RelativeTime value={n.createdAt} /></p>
                  <p className="whitespace-pre-line text-sm text-fg">{n.text}</p>
                </li>
              ))}
            </ol>
          )}
          {canNote && !notes.isError && (
            <form onSubmit={submit} className="mt-3 space-y-2">
              <label htmlFor={`${assigneeId}-note`} className="sr-only">Add a note</label>
              <textarea
                ref={noteRef}
                id={`${assigneeId}-note`}
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submit(e); }}
                rows={3}
                maxLength={NOTE_MAX}
                placeholder="What did you find or do? (Ctrl+Enter to add)"
                className="w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary"
              />
              <div className="flex items-center justify-between">
                <span className="text-xs text-fg-muted tabular-nums">{text.length}/{NOTE_MAX}</span>
                <Button type="submit" size="sm" icon="add_comment" disabled={!text.trim() || addNote.isPending}>Add note</Button>
              </div>
            </form>
          )}
        </section>
      </div>
    </Dialog>
  );
}
