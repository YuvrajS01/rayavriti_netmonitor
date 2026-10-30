import { useState, useMemo, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import LocationTree from '../components/LocationTree';
import type { ResourceRow } from '../api/resources';
import { getDevices } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import { useResourceList, useResourceMutations } from '../hooks/useResources';
import { useToast } from '../components/ui/useToast';
import { asText } from '../utils/formatters';
import LoadingState from '../components/ui/LoadingState';
import { StatusDot } from '../components/ui/Status';

const locationTypes = ['campus', 'building', 'floor', 'room', 'rack', 'zone'] as const;

const emptyForm = {
  name: '',
  type: 'building' as string,
  code: '',
  parent_id: '' as string,
  description: '',
  floor_number: '' as string,
  enabled: false,
};

const EMPTY_LOCATIONS: ResourceRow[] = [];

export default function LocationManager() {
  const { addToast } = useToast();
  const [selected, setSelected] = useState<ResourceRow | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [isCreating, setIsCreating] = useState(false);

  const locationsQuery = useResourceList('/locations');
  const locations = locationsQuery.data ?? EMPTY_LOCATIONS;
  const loading = locationsQuery.isPending;
  const { save, remove } = useResourceMutations('/locations');
  const saving = save.isPending;

  const selectedId = selected ? Number(selected.id) : null;
  const devicesQuery = useQuery({
    queryKey: queryKeys.devices.list(),
    queryFn: ({ signal }) => getDevices({ signal }),
    enabled: selectedId != null,
  });
  const devices = useMemo(
    () => (selectedId == null ? [] : (devicesQuery.data?.data || []).filter((d) => d.locationId === selectedId)),
    [devicesQuery.data, selectedId],
  );

  const handleSelect = useCallback((loc: ResourceRow) => {
    setSelected(loc);
    setIsCreating(false);
    setForm({
      name: asText(loc.name),
      type: asText(loc.type, 'building'),
      code: asText(loc.code),
      parent_id: loc.parent_id != null ? asText(loc.parent_id) : '',
      description: asText(loc.description),
      floor_number: loc.floor_number != null ? asText(loc.floor_number) : '',
      enabled: loc.enabled !== false,
    });
  }, []);

  const handleNew = useCallback(() => {
    setSelected(null);
    setIsCreating(true);
    setForm({ ...emptyForm, parent_id: '' });
  }, []);

  const handleSave = () => {
    if (!form.name.trim()) {
      addToast('Name is required', 'error');
      return;
    }
    const payload: Record<string, unknown> = {
      name: form.name.trim(),
      type: form.type,
      code: form.code.trim() || null,
      parent_id: form.parent_id ? Number(form.parent_id) : null,
      description: form.description.trim(),
      floor_number: form.floor_number ? Number(form.floor_number) : null,
      enabled: form.enabled,
    };
    if (!isCreating && !selected) return;
    const creating = isCreating;
    save.mutate({ id: creating ? undefined : Number(selected?.id), body: payload }, {
      onSuccess: () => {
        addToast(creating ? 'Location created' : 'Location updated', 'success');
        if (creating) {
          setIsCreating(false);
          setForm(emptyForm);
        }
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Save failed', 'error'),
    });
  };

  const handleDelete = () => {
    if (!selected) return;
    if (!window.confirm(`Delete "${asText(selected.name)}"? Children will be moved to its parent.`)) return;
    remove.mutate(Number(selected.id), {
      onSuccess: () => {
        addToast('Location deleted', 'success');
        setSelected(null);
        setForm(emptyForm);
      },
      onError: (err) => addToast(err instanceof Error ? err.message : 'Delete failed', 'error'),
    });
  };

  // Parent options (exclude self and descendants for editing).
  const parentOptions = useMemo(() => {
    const selfId = selected ? Number(selected.id) : -1;
    return locations.filter((l) => Number(l.id) !== selfId);
  }, [locations, selected]);

  const isEditing = selected && !isCreating;

  if (loading) {
    return (
      <LoadingState message="Loading locations…" />
    );
  }

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Location manager"
        subtitle="Create and organize your campus hierarchy — buildings, floors, rooms, and racks."
        action={
          <Button icon="add" onClick={handleNew}>
            Add location
          </Button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Left: Tree */}
        <Card variant="low" className="lg:col-span-4 xl:col-span-3 p-4 max-h-[75vh] overflow-hidden flex flex-col">
          <h2 className="font-semibold text-sm text-fg mb-3">
            Hierarchy
          </h2>
          {locations.length === 0 ? (
            <EmptyState
              icon="account_tree"
              title="No locations"
              description="Choose Add location to create your first one."
            />
          ) : (
            <LocationTree
              locations={locations}
              onSelect={handleSelect}
              selectedId={selected ? Number(selected.id) : null}
            />
          )}
        </Card>

        {/* Right: Form */}
        <Card variant="low" className="lg:col-span-8 xl:col-span-9 p-0 overflow-hidden">
          {isCreating || isEditing ? (
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="font-semibold text-lg">
                  {isCreating ? 'New location' : `Edit ${String(selected?.name)}`}
                </h2>
                {isEditing && (
                  <Button variant="danger-outline" icon="delete" onClick={handleDelete}>
                    Delete
                  </Button>
                )}
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                {/* Name */}
                <div>
                  <label htmlFor="loc-name" className="block text-xs text-fg-muted font-medium mb-1.5">
                    Name *
                  </label>
                  <input
                    id="loc-name"
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    placeholder="e.g. Admin Block"
                    className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none"
                  />
                </div>

                {/* Type */}
                <div>
                  <label htmlFor="loc-type" className="block text-xs text-fg-muted font-medium mb-1.5">
                    Type
                  </label>
                  <select
                    id="loc-type"
                    value={form.type}
                    onChange={(e) => setForm((f) => ({ ...f, type: e.target.value }))}
                    className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg focus:border-primary outline-none"
                  >
                    {locationTypes.map((t) => (
                      <option key={t} value={t}>
                        {t.charAt(0).toUpperCase() + t.slice(1)}
                      </option>
                    ))}
                  </select>
                </div>

                {/* Code */}
                <div>
                  <label htmlFor="loc-code" className="block text-xs text-fg-muted font-medium mb-1.5">
                    Code
                  </label>
                  <input
                    id="loc-code"
                    value={form.code}
                    onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))}
                    placeholder="e.g. AB-GF-101"
                    className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none"
                  />
                </div>

                {/* Parent */}
                <div>
                  <label htmlFor="loc-parent" className="block text-xs text-fg-muted font-medium mb-1.5">
                    Parent location
                  </label>
                  <select
                    id="loc-parent"
                    value={form.parent_id}
                    onChange={(e) => setForm((f) => ({ ...f, parent_id: e.target.value }))}
                    className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg focus:border-primary outline-none"
                  >
                    <option value="">None (root)</option>
                    {parentOptions.map((l) => (
                      <option key={Number(l.id)} value={String(l.id)}>
                        {String(l.name)} ({String(l.type)})
                      </option>
                    ))}
                  </select>
                </div>

                {/* Floor Number (only for floor type) */}
                {form.type === 'floor' && (
                  <div>
                    <label htmlFor="loc-floor" className="block text-xs text-fg-muted font-medium mb-1.5">
                      Floor number
                    </label>
                    <input
                      id="loc-floor"
                      type="number"
                      value={form.floor_number}
                      onChange={(e) => setForm((f) => ({ ...f, floor_number: e.target.value }))}
                      placeholder="0"
                      className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none"
                    />
                  </div>
                )}

                {/* Description — full width */}
                <div className="md:col-span-2">
                  <label htmlFor="loc-description" className="block text-xs text-fg-muted font-medium mb-1.5">
                    Description
                  </label>
                  <textarea
                    id="loc-description"
                    value={form.description}
                    onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                    placeholder="Optional description…"
                    rows={3}
                    className="w-full bg-inset border border-line rounded-md px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none resize-none"
                  />
                </div>

                {/* Enabled toggle */}
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    role="switch"
                    aria-checked={form.enabled}
                    aria-labelledby="loc-enabled-label"
                    onClick={() => setForm((f) => ({ ...f, enabled: !f.enabled }))}
                    className={[
                      'relative w-11 h-6 rounded-full overflow-hidden transition-colors duration-200',
                      form.enabled ? 'bg-primary' : 'bg-surface-3 ring-1 ring-inset ring-line-strong',
                    ].join(' ')}
                  >
                    <span
                      className={[
                        'absolute left-0.5 top-0.5 w-5 h-5 rounded-full transition-transform duration-[var(--duration-fast)] shadow-e1',
                        form.enabled ? 'bg-on-primary' : 'bg-fg-muted',
                        form.enabled ? 'translate-x-[18px]' : 'translate-x-0',
                      ].join(' ')}
                    />
                  </button>
                  <span id="loc-enabled-label" className="text-sm text-fg-muted">
                    {form.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-3 mt-8 pt-5 border-t border-line">
                <Button icon="save" onClick={handleSave} disabled={saving}>
                  {saving ? 'Saving…' : isCreating ? 'Create' : 'Save changes'}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    setIsCreating(false);
                    setSelected(null);
                    setForm(emptyForm);
                  }}
                >
                  Cancel
                </Button>
              </div>

              {/* Assigned Devices */}
              {isEditing && (
                <div className="mt-6 pt-5 border-t border-line">
                  <h3 className="font-semibold text-sm text-fg mb-3">
                    Devices at this location
                  </h3>
                  {devices.length === 0 ? (
                    <p className="text-sm text-fg-muted">No devices assigned to this location.</p>
                  ) : (
                    <div className="space-y-2">
                      {devices.map((dev) => (
                        <div key={dev.id} className="flex items-center justify-between bg-inset rounded-md px-4 py-2.5 border border-line">
                          <div className="flex items-center gap-3 min-w-0">
                            <StatusDot status={dev.status} />
                            <div className="min-w-0">
                              <p className="text-sm font-semibold text-fg truncate">{dev.name}</p>
                              <p className="text-xs text-fg-muted font-mono">{dev.ipAddress}</p>
                            </div>
                          </div>
                          <span className="text-xs font-semibold uppercase text-fg-muted shrink-0 ml-3">{dev.protocol}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center min-h-[400px] text-fg-muted">
              <span className="material-symbols-outlined text-5xl mb-3 text-fg-subtle" aria-hidden="true">edit_location_alt</span>
              <p className="font-semibold text-fg">Select or create a location</p>
              <p className="text-sm mt-1">Choose from the tree or use Add location to get started.</p>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
