import { Suspense, useEffect, useId, useMemo, useState, type FormEvent } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { createDashboard, deleteDashboard, getDashboards, setDefaultDashboard, updateDashboard, type DashboardInput } from '../api/dashboards';
import { queryKeys } from '../api/queryKeys';
import type { Dashboard as SavedDashboard, DashboardLayout, DashboardWidget } from '../api/types';
import DashboardGrid from '../components/dashboard/DashboardGrid';
import { DashboardSkeleton } from '../components/dashboard/DashboardSkeleton';
import { LAYOUT_LIMITS, OVERVIEW_LAYOUT, addWidget, cloneLayout, emptyLayout, layoutProblem, normalizeLayout, updateWidget } from '../components/dashboard/layout';
import DashboardLiveProvider from '../components/dashboard/DashboardLiveProvider';
import { useLatestMetrics } from '../components/dashboard/useDashboardData';
import type { WidgetMeta } from '../components/dashboard/widgets/catalog';
import Button from '../components/ui/Button';
import Dialog from '../components/ui/Dialog';
import EmptyState from '../components/ui/EmptyState';
import Popover from '../components/ui/Popover';
import SectionHeader from '../components/ui/SectionHeader';
import { useToast } from '../components/ui/useToast';
import { hasPermission } from '../navigation';
import { useSessionUser } from '../store/session';
import { lazyWithRetry } from '../utils/lazyWithRetry';

const WidgetCatalogDialog = lazyWithRetry(() => import('../components/dashboard/WidgetCatalogDialog'));
const WidgetSettingsDialog = lazyWithRetry(() => import('../components/dashboard/widgets/WidgetSettingsDialog'));

const OVERVIEW_NAME = 'Overview';
const EMPTY: SavedDashboard[] = [];

interface Draft {
  name: string;
  layout: DashboardLayout;
}

// Unsaved edits survive leaving the page (the router can't block in-app
// navigation), and are offered again on return.
const draftKey = (id: number) => `nm-dashboard-draft:${id}`;
function loadDraft(id: number): Draft | null {
  try {
    const raw = sessionStorage.getItem(draftKey(id));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<Draft>;
    return typeof parsed.name === 'string' ? { name: parsed.name, layout: normalizeLayout(parsed.layout) } : null;
  } catch {
    return null;
  }
}
function storeDraft(id: number, draft: Draft | null) {
  try {
    if (draft) sessionStorage.setItem(draftKey(id), JSON.stringify(draft));
    else sessionStorage.removeItem(draftKey(id));
  } catch {
    /* storage unavailable: the draft just won't survive navigation */
  }
}

/** The server's error message (e.g. a layout validation error), else the fallback. */
function errorMessage(err: unknown, fallback: string): string {
  if (isAxiosError<{ error?: { message?: unknown } }>(err)) {
    const message = err.response?.data?.error?.message;
    if (typeof message === 'string') return message;
  }
  return fallback;
}

type NameDialogState =
  | { kind: 'new' }
  | { kind: 'rename'; dashboard: SavedDashboard }
  | { kind: 'duplicate'; name: string; layout: DashboardLayout };

export default function Dashboard() {
  const { id: param } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const user = useSessionUser();
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const list = useQuery({ queryKey: queryKeys.dashboards.list(), queryFn: ({ signal }) => getDashboards({ signal }) });
  const dashboards = list.data ?? EMPTY;
  // Keep the overview's data warm while the dashboard list loads.
  useLatestMetrics();

  // Which dashboard to show: an explicit one, else the user's default, else
  // the built-in overview. null = built-in.
  const numericId = param && param !== 'overview' ? Number(param) : null;
  const saved: SavedDashboard | null | undefined = useMemo(() => {
    if (param === 'overview') return null;
    if (numericId != null) return dashboards.find((d) => d.id === numericId);
    return dashboards.find((d) => d.isDefault) ?? null;
  }, [param, numericId, dashboards]);
  const layout = useMemo(() => (saved ? normalizeLayout(saved.layout) : OVERVIEW_LAYOUT), [saved]);
  const name = saved?.name ?? OVERVIEW_NAME;
  const isOwner = !!saved && saved.userId === user?.id;
  const canShare = hasPermission(user, 'devices.write');

  const [draft, setDraft] = useState<Draft | null>(null);
  const [restored, setRestored] = useState(false);
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [configuring, setConfiguring] = useState<DashboardWidget | null>(null);
  const [nameDialog, setNameDialog] = useState<NameDialogState | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [saveError, setSaveError] = useState('');

  // When the dashboard changes, pick up a stored draft, or start editing when
  // arriving from "duplicate". Done during render (not in an effect) so the
  // first paint already shows the right mode.
  const editKey = isOwner && saved ? saved.id : null;
  const [draftFor, setDraftFor] = useState<number | null | 'unset'>('unset');
  if (draftFor !== editKey) {
    setDraftFor(editKey);
    setSaveError('');
    const stored = editKey != null ? loadDraft(editKey) : null;
    const fromDuplicate = editKey != null && (location.state as { edit?: boolean } | null)?.edit === true;
    setDraft(stored ?? (fromDuplicate ? { name, layout } : null));
    setRestored(!!stored);
  }
  const savedId = saved?.id;

  const dirty = !!draft && (draft.name !== name || JSON.stringify(draft.layout) !== JSON.stringify(layout));
  useEffect(() => {
    if (savedId) storeDraft(savedId, dirty ? draft : null);
  }, [savedId, draft, dirty]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.dashboards.list() });
  const upsertCached = (d: SavedDashboard) =>
    queryClient.setQueryData<SavedDashboard[]>(queryKeys.dashboards.list(), (prev) => (prev?.some((x) => x.id === d.id) ? prev.map((x) => (x.id === d.id ? d : x)) : [...(prev ?? []), d]));

  const save = useMutation({
    mutationFn: ({ id, input }: { id: number; input: DashboardInput }) => updateDashboard(id, input),
    onSuccess: (d) => {
      upsertCached(d);
      void invalidate();
    },
  });
  const create = useMutation({ mutationFn: createDashboard, onSuccess: () => void invalidate() });
  const remove = useMutation({ mutationFn: deleteDashboard, onSuccess: () => void invalidate() });
  const makeDefault = useMutation({ mutationFn: setDefaultDashboard, onSuccess: () => void invalidate() });

  const saveDraft = () => {
    if (!saved || !draft) return;
    const problem = layoutProblem(draft.layout);
    if (problem) {
      setSaveError(problem);
      return;
    }
    setSaveError('');
    save.mutate(
      { id: saved.id, input: { name: draft.name, layout: draft.layout } },
      {
        onSuccess: () => {
          storeDraft(saved.id, null);
          setDraft(null);
          setRestored(false);
          addToast('Dashboard saved', 'success');
        },
        onError: (err) => setSaveError(errorMessage(err, 'Saving failed. Try again.')),
      },
    );
  };
  const cancelEdit = () => {
    if (saved) storeDraft(saved.id, null);
    setDraft(null);
    setRestored(false);
    setSaveError('');
  };

  const addFromCatalog = (meta: WidgetMeta) => {
    if (!draft) return;
    const next = addWidget(draft.layout, { type: meta.type, ...meta.defaultSize, config: meta.defaultConfig ? { ...meta.defaultConfig } : undefined });
    setDraft({ ...draft, layout: next });
    setCatalogOpen(false);
    // Configurable widgets need settings before they show anything.
    if (meta.configurable) setConfiguring(next.widgets[next.widgets.length - 1]!);
  };

  const submitName = (value: string, startFrom?: 'blank' | 'overview') => {
    const dialog = nameDialog;
    if (!dialog) return;
    if (dialog.kind === 'rename') {
      save.mutate(
        { id: dialog.dashboard.id, input: { name: value } },
        { onSuccess: () => { setNameDialog(null); addToast('Dashboard renamed', 'success'); }, onError: (err) => addToast(errorMessage(err, 'Renaming failed'), 'error') },
      );
      return;
    }
    const source = dialog.kind === 'duplicate' ? dialog.layout : startFrom === 'overview' ? OVERVIEW_LAYOUT : emptyLayout();
    create.mutate(
      { name: value, layout: cloneLayout(source) },
      {
        onSuccess: (d) => {
          setNameDialog(null);
          upsertCached(d);
          void navigate(`/dashboards/${d.id}`, { state: { edit: true } });
        },
        onError: (err) => addToast(errorMessage(err, 'Creating the dashboard failed'), 'error'),
      },
    );
  };

  if (list.isPending && param !== 'overview') return <DashboardSkeleton />;
  if (numericId != null && !saved && list.isSuccess) {
    return (
      <EmptyState
        icon="dashboard"
        title="Dashboard not found"
        description="It may have been deleted, or its owner stopped sharing it."
        action={<Link to="/" className="text-sm text-primary underline">Go to your dashboard</Link>}
      />
    );
  }

  const editing = !!draft;
  const shownLayout = draft?.layout ?? layout;
  const subtitle = editing
    ? `${shownLayout.widgets.length} of ${LAYOUT_LIMITS.maxWidgets} widgets. Drag widgets, or use the arrows to move them.`
    : saved
      ? saved.shared
        ? isOwner ? 'Shared with everyone, read-only for others.' : `Shared by ${saved.ownerName || 'another user'}. Duplicate it to make your own version.`
        : 'Only you can see this dashboard.'
      : 'The built-in overview. Duplicate it to customise.';

  return (
    <DashboardLiveProvider>
      <SectionHeader
        title={editing ? `Editing ${draft.name}` : name}
        subtitle={subtitle}
        action={
          editing ? (
            <div className="flex flex-wrap items-center gap-2">
              <Button variant="secondary" icon="add" onClick={() => setCatalogOpen(true)}>Add widget</Button>
              <Button variant="ghost" onClick={() => setNameDialog({ kind: 'rename', dashboard: saved! })} disabled={!saved}>Rename</Button>
              <Button variant="secondary" onClick={cancelEdit}>Cancel</Button>
              <Button icon="check" onClick={saveDraft} disabled={save.isPending || !dirty}>{save.isPending ? 'Saving…' : 'Save'}</Button>
            </div>
          ) : (
            <div className="flex flex-wrap items-center gap-2">
              <DashboardSwitcher dashboards={dashboards} current={saved ?? null} userId={user?.id} />
              {isOwner ? (
                <Button variant="secondary" icon="edit" onClick={() => setDraft({ name, layout })}>Edit</Button>
              ) : (
                <Button variant="secondary" icon="content_copy" onClick={() => setNameDialog({ kind: 'duplicate', name: `${name} (copy)`, layout })}>
                  {saved ? 'Duplicate to edit' : 'Customise'}
                </Button>
              )}
              <Popover
                label="Dashboard actions"
                trigger={(props) => (
                  <button type="button" {...props} aria-label="More dashboard actions" className="grid h-10 w-10 place-items-center rounded-md border border-line bg-surface-2 text-fg hover:bg-surface-3">
                    <span className="material-symbols-outlined text-[20px]" aria-hidden="true">more_vert</span>
                  </button>
                )}
              >
                {(close) => (
                  <ul className="w-60 py-1 text-sm">
                    <MenuItem icon="add" onClick={() => { close(); setNameDialog({ kind: 'new' }); }}>New dashboard</MenuItem>
                    <MenuItem icon="content_copy" onClick={() => { close(); setNameDialog({ kind: 'duplicate', name: `${name} (copy)`, layout }); }}>Duplicate</MenuItem>
                    {isOwner && <MenuItem icon="edit" onClick={() => { close(); setNameDialog({ kind: 'rename', dashboard: saved }); }}>Rename</MenuItem>}
                    {saved?.isDefault ? (
                      <MenuItem icon="home" onClick={() => { close(); makeDefault.mutate(null, { onSuccess: () => addToast('The built-in overview opens first again', 'success') }); }}>Stop opening first</MenuItem>
                    ) : saved ? (
                      <MenuItem icon="home" onClick={() => { close(); makeDefault.mutate(saved.id, { onSuccess: () => addToast(`${saved.name} now opens first`, 'success') }); }}>Open this first</MenuItem>
                    ) : dashboards.some((d) => d.isDefault) ? (
                      <MenuItem icon="home" onClick={() => { close(); makeDefault.mutate(null, { onSuccess: () => addToast('The overview now opens first', 'success') }); }}>Open this first</MenuItem>
                    ) : null}
                    {isOwner && canShare && (
                      <MenuItem
                        icon={saved.shared ? 'lock' : 'group'}
                        onClick={() => {
                          close();
                          save.mutate(
                            { id: saved.id, input: { shared: !saved.shared } },
                            { onSuccess: (d) => addToast(d.shared ? 'Shared with everyone (read-only)' : 'Now private', 'success'), onError: (err) => addToast(errorMessage(err, 'Sharing failed'), 'error') },
                          );
                        }}
                      >
                        {saved.shared ? 'Stop sharing' : 'Share with everyone'}
                      </MenuItem>
                    )}
                    {isOwner && <MenuItem icon="delete" danger onClick={() => { close(); setConfirmDelete(true); }}>Delete</MenuItem>}
                  </ul>
                )}
              </Popover>
            </div>
          )
        }
      />

      {restored && editing && (
        <div role="status" className="mb-4 flex flex-wrap items-center gap-3 rounded-md border border-line bg-surface-2 px-4 py-2.5 text-sm text-fg">
          <span className="material-symbols-outlined text-[20px] text-primary" aria-hidden="true">history</span>
          <span className="flex-1">You have unsaved changes from earlier. Save them or cancel to discard.</span>
        </div>
      )}
      {saveError && <p role="alert" className="mb-4 rounded-md border border-error/40 bg-error/10 px-4 py-2.5 text-sm text-error">{saveError}</p>}

      {shownLayout.widgets.length === 0 ? (
        <EmptyState
          icon="dashboard_customize"
          title="This dashboard is empty"
          description={editing ? 'Add widgets to build it.' : isOwner ? 'Edit it to add widgets.' : 'Its owner hasn’t added widgets yet.'}
          action={editing ? <Button icon="add" onClick={() => setCatalogOpen(true)}>Add widget</Button> : isOwner ? <Button icon="edit" onClick={() => setDraft({ name, layout })}>Edit</Button> : undefined}
        />
      ) : (
        <DashboardGrid
          layout={shownLayout}
          editing={editing}
          onChange={(next) => draft && setDraft({ ...draft, layout: next })}
          onConfigure={setConfiguring}
        />
      )}

      <Suspense fallback={null}>
        {catalogOpen && draft && <WidgetCatalogDialog layout={draft.layout} onClose={() => setCatalogOpen(false)} onAdd={addFromCatalog} />}
        {configuring && draft && (
          <WidgetSettingsDialog
            widget={configuring}
            onClose={() => setConfiguring(null)}
            onSave={(config) => {
              setDraft({ ...draft, layout: updateWidget(draft.layout, configuring.id, { config }) });
              setConfiguring(null);
            }}
          />
        )}
      </Suspense>

      {nameDialog && (
        <NameDialog
          state={nameDialog}
          busy={create.isPending || save.isPending}
          onClose={() => setNameDialog(null)}
          onSubmit={(value, startFrom) => {
            if (nameDialog.kind === 'rename' && editing) {
              setDraft({ ...draft, name: value });
              setNameDialog(null);
              return;
            }
            submitName(value, startFrom);
          }}
        />
      )}

      <Dialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        role="alertdialog"
        title={`Delete ${name}?`}
        description={saved?.shared ? 'It is shared, so it will disappear for everyone. This can’t be undone.' : 'This can’t be undone.'}
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmDelete(false)}>Cancel</Button>
            <Button
              variant="danger"
              disabled={remove.isPending}
              onClick={() =>
                saved &&
                remove.mutate(saved.id, {
                  onSuccess: () => {
                    storeDraft(saved.id, null);
                    setConfirmDelete(false);
                    addToast('Dashboard deleted', 'success');
                    void navigate('/');
                  },
                  onError: (err) => addToast(errorMessage(err, 'Deleting failed'), 'error'),
                })
              }
            >
              Delete dashboard
            </Button>
          </>
        }
      />
    </DashboardLiveProvider>
  );
}

function MenuItem({ icon, children, onClick, danger = false }: { icon: string; children: string; onClick: () => void; danger?: boolean }) {
  return (
    <li>
      <button type="button" onClick={onClick} className={`flex w-full items-center gap-2.5 px-3 py-2 text-left hover:bg-surface-2 ${danger ? 'text-error' : 'text-fg'}`}>
        <span className="material-symbols-outlined text-[18px]" aria-hidden="true">{icon}</span>
        {children}
      </button>
    </li>
  );
}

/** Lists the built-in overview, the user's dashboards and shared ones. */
function DashboardSwitcher({ dashboards, current, userId }: { dashboards: SavedDashboard[]; current: SavedDashboard | null; userId?: number }) {
  const own = dashboards.filter((d) => d.userId === userId);
  const shared = dashboards.filter((d) => d.userId !== userId);
  const item = (to: string, label: string, active: boolean, close: () => void, meta?: string, isDefault?: boolean) => (
    <li key={to}>
      <Link
        to={to}
        onClick={close}
        aria-current={active ? 'page' : undefined}
        className={`flex items-center gap-2 px-3 py-2 text-sm hover:bg-surface-2 ${active ? 'font-semibold text-primary' : 'text-fg'}`}
      >
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {isDefault && <span className="rounded-full bg-surface-3 px-2 py-0.5 text-xs text-fg-muted">Opens first</span>}
        {meta && <span className="truncate text-xs text-fg-muted">{meta}</span>}
      </Link>
    </li>
  );
  return (
    <Popover
      label="Switch dashboard"
      align="start"
      trigger={(props) => (
        <Button {...props} variant="secondary" icon="dashboard">
          Dashboards
        </Button>
      )}
    >
      {(close) => (
        <div className="max-h-96 w-72 overflow-y-auto py-1">
          <ul>{item('/dashboards/overview', OVERVIEW_NAME, !current, close, 'Built-in', !dashboards.some((d) => d.isDefault))}</ul>
          {own.length > 0 && (
            <>
              <p className="px-3 pb-1 pt-3 text-xs font-semibold text-fg-muted">Yours</p>
              <ul>{own.map((d) => item(`/dashboards/${d.id}`, d.name, current?.id === d.id, close, d.shared ? 'Shared' : undefined, d.isDefault))}</ul>
            </>
          )}
          {shared.length > 0 && (
            <>
              <p className="px-3 pb-1 pt-3 text-xs font-semibold text-fg-muted">Shared with you</p>
              <ul>{shared.map((d) => item(`/dashboards/${d.id}`, d.name, current?.id === d.id, close, d.ownerName, d.isDefault))}</ul>
            </>
          )}
        </div>
      )}
    </Popover>
  );
}

function NameDialog({ state, busy, onClose, onSubmit }: { state: NameDialogState; busy: boolean; onClose: () => void; onSubmit: (name: string, startFrom?: 'blank' | 'overview') => void }) {
  const initial = state.kind === 'rename' ? state.dashboard.name : state.kind === 'duplicate' ? state.name : '';
  const [value, setValue] = useState(initial);
  const [startFrom, setStartFrom] = useState<'blank' | 'overview'>('overview');
  const [error, setError] = useState('');
  const nameId = useId();
  const title = state.kind === 'new' ? 'New dashboard' : state.kind === 'rename' ? 'Rename dashboard' : 'Duplicate dashboard';

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const name = value.trim();
    if (!name) return setError('Give the dashboard a name.');
    if (name.length > LAYOUT_LIMITS.maxNameLength) return setError(`Use at most ${LAYOUT_LIMITS.maxNameLength} characters.`);
    onSubmit(name, state.kind === 'new' ? startFrom : undefined);
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={title}
      description={state.kind === 'duplicate' ? 'The copy is yours to edit and starts private.' : undefined}
      size="sm"
      footer={
        <>
          <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
          <Button type="submit" form="dashboard-name-form" disabled={busy}>{state.kind === 'rename' ? 'Rename' : 'Create'}</Button>
        </>
      }
    >
      <form id="dashboard-name-form" onSubmit={submit} className="space-y-4" noValidate>
        <div>
          <label htmlFor={nameId} className="mb-1.5 block text-xs font-semibold text-fg-muted">Name</label>
          <input
            id={nameId}
            value={value}
            onChange={(e) => { setValue(e.target.value); setError(''); }}
            maxLength={LAYOUT_LIMITS.maxNameLength}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? `${nameId}-error` : undefined}
            className={`w-full rounded-md border bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary ${error ? 'border-error' : 'border-line'}`}
          />
          {error && <p id={`${nameId}-error`} className="mt-1 text-xs text-error">{error}</p>}
        </div>
        {state.kind === 'new' && (
          <fieldset>
            <legend className="mb-1.5 text-xs font-semibold text-fg-muted">Start from</legend>
            <div className="space-y-2 text-sm text-fg">
              <label className="flex items-center gap-2"><input type="radio" name="start" checked={startFrom === 'overview'} onChange={() => setStartFrom('overview')} className="accent-[var(--color-primary)]" /> A copy of the overview</label>
              <label className="flex items-center gap-2"><input type="radio" name="start" checked={startFrom === 'blank'} onChange={() => setStartFrom('blank')} className="accent-[var(--color-primary)]" /> A blank dashboard</label>
            </div>
          </fieldset>
        )}
      </form>
    </Dialog>
  );
}
