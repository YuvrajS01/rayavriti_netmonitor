import { useEffect, useState, useMemo, useCallback } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import LocationTree from '../components/LocationTree';
import { listPhase2, createPhase2, updatePhase2, type Phase2Row } from '../api/phase2';
import { v1 } from '../api/http';
import { getDevices } from '../api/client';
import { useToast } from '../components/ui/useToast';
import type { Device } from '../api/types';

const locationTypes = ['campus', 'building', 'floor', 'room', 'rack', 'zone'] as const;

const emptyForm = {
  name: '',
  type: 'building' as string,
  code: '',
  parent_id: '' as string,
  description: '',
  floor_number: '' as string,
  enabled: true,
};

export default function LocationManager() {
  const { addToast } = useToast();
  const [locations, setLocations] = useState<Phase2Row[]>([]);
  const [selected, setSelected] = useState<Phase2Row | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [isCreating, setIsCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [devices, setDevices] = useState<Device[]>([]);

  const loadData = async () => {
    setLoading(true);
    try {
      const res = await listPhase2('/locations');
      setLocations(res.data || []);
    } catch {
      setLocations([]);
    }
    setLoading(false);
  };

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const res = await listPhase2('/locations');
        if (active) setLocations(res.data || []);
      } catch {
        if (active) setLocations([]);
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, []);

  const handleSelect = useCallback((loc: Phase2Row) => {
    setSelected(loc);
    setIsCreating(false);
    setForm({
      name: String(loc.name ?? ''),
      type: String(loc.type ?? 'building'),
      code: String(loc.code ?? ''),
      parent_id: loc.parent_id != null ? String(loc.parent_id) : '',
      description: String(loc.description ?? ''),
      floor_number: loc.floor_number != null ? String(loc.floor_number) : '',
      enabled: loc.enabled !== false,
    });
    getDevices().then((res) => {
      const all = res.data || [];
      setDevices(all.filter((d) => d.locationId === Number(loc.id)));
    }).catch(() => setDevices([]));
  }, []);

  const handleNew = useCallback(() => {
    setSelected(null);
    setIsCreating(true);
    setForm({ ...emptyForm, parent_id: '' });
  }, []);

  const handleSave = async () => {
    if (!form.name.trim()) {
      addToast('Name is required', 'error');
      return;
    }
    setSaving(true);
    try {
      const payload: Record<string, unknown> = {
        name: form.name.trim(),
        type: form.type,
        code: form.code.trim() || null,
        parent_id: form.parent_id ? Number(form.parent_id) : null,
        description: form.description.trim(),
        floor_number: form.floor_number ? Number(form.floor_number) : null,
        enabled: form.enabled,
      };

      if (isCreating) {
        await createPhase2('/locations', payload);
        addToast('Location created', 'success');
      } else if (selected) {
        await updatePhase2('/locations', Number(selected.id), payload);
        addToast('Location updated', 'success');
      }
      await loadData();
      if (isCreating) {
        setIsCreating(false);
        setForm(emptyForm);
      }
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Save failed', 'error');
    }
    setSaving(false);
  };

  const handleDelete = async () => {
    if (!selected) return;
    if (!window.confirm(`Delete "${selected.name}"? Children will be moved to its parent.`)) return;
    try {
      await v1.delete(`/locations/${selected.id}`);
      addToast('Location deleted', 'success');
      setSelected(null);
      setForm(emptyForm);
      await loadData();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Delete failed', 'error');
    }
  };

  // Parent options (exclude self and descendants for editing).
  const parentOptions = useMemo(() => {
    const selfId = selected ? Number(selected.id) : -1;
    return locations.filter((l) => Number(l.id) !== selfId);
  }, [locations, selected]);

  const isEditing = selected && !isCreating;

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Location Manager"
        subtitle="Create and organize your campus hierarchy — buildings, floors, rooms, and racks."
        action={
          <Button icon="add" onClick={handleNew}>
            Add Location
          </Button>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Left: Tree */}
        <Card variant="low" className="lg:col-span-4 xl:col-span-3 p-4 max-h-[75vh] overflow-hidden flex flex-col">
          <h2 className="font-headline font-bold text-sm uppercase tracking-wide text-on-surface-variant mb-3">
            Hierarchy
          </h2>
          {locations.length === 0 ? (
            <EmptyState
              icon="account_tree"
              title="No locations"
              description="Click Add Location to create your first one."
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
                <h2 className="font-headline font-bold text-lg">
                  {isCreating ? 'New Location' : `Edit: ${String(selected?.name)}`}
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
                  <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                    Name *
                  </label>
                  <input
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    placeholder="e.g. Admin Block"
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
                  />
                </div>

                {/* Type */}
                <div>
                  <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                    Type
                  </label>
                  <select
                    value={form.type}
                    onChange={(e) => setForm((f) => ({ ...f, type: e.target.value }))}
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface focus:ring-1 focus:ring-primary outline-none appearance-none"
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
                  <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                    Code
                  </label>
                  <input
                    value={form.code}
                    onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))}
                    placeholder="e.g. AB-GF-101"
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
                  />
                </div>

                {/* Parent */}
                <div>
                  <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                    Parent Location
                  </label>
                  <select
                    value={form.parent_id}
                    onChange={(e) => setForm((f) => ({ ...f, parent_id: e.target.value }))}
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface focus:ring-1 focus:ring-primary outline-none appearance-none"
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
                    <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                      Floor Number
                    </label>
                    <input
                      type="number"
                      value={form.floor_number}
                      onChange={(e) => setForm((f) => ({ ...f, floor_number: e.target.value }))}
                      placeholder="0"
                      className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none"
                    />
                  </div>
                )}

                {/* Description — full width */}
                <div className="md:col-span-2">
                  <label className="block text-[10px] uppercase tracking-wide text-on-surface-variant font-bold mb-1.5">
                    Description
                  </label>
                  <textarea
                    value={form.description}
                    onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                    placeholder="Optional description..."
                    rows={3}
                    className="w-full bg-surface-container-highest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none resize-none"
                  />
                </div>

                {/* Enabled toggle */}
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    role="switch"
                    aria-checked={form.enabled}
                    onClick={() => setForm((f) => ({ ...f, enabled: !f.enabled }))}
                    className={[
                      'relative w-11 h-6 rounded-full transition-colors duration-200',
                      form.enabled ? 'bg-primary' : 'bg-outline-variant/40',
                    ].join(' ')}
                  >
                    <span
                      className={[
                        'absolute top-0.5 w-5 h-5 rounded-full bg-on-primary transition-transform duration-200 shadow',
                        form.enabled ? 'translate-x-[22px]' : 'translate-x-0.5',
                      ].join(' ')}
                    />
                  </button>
                  <span className="text-sm text-on-surface-variant">
                    {form.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
              </div>

              {/* Actions */}
              <div className="flex items-center gap-3 mt-8 pt-5 border-t border-outline-variant/20">
                <Button icon="save" onClick={handleSave} disabled={saving}>
                  {saving ? 'Saving...' : isCreating ? 'Create' : 'Save Changes'}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    setIsCreating(false);
                    setSelected(null);
                    setForm(emptyForm);
                    setDevices([]);
                  }}
                >
                  Cancel
                </Button>
              </div>

              {/* Assigned Devices */}
              {isEditing && (
                <div className="mt-6 pt-5 border-t border-outline-variant/20">
                  <h3 className="font-headline font-bold text-sm uppercase tracking-wide text-on-surface-variant mb-3">
                    Devices at this Location
                  </h3>
                  {devices.length === 0 ? (
                    <p className="text-xs text-on-surface-variant">No devices assigned to this location.</p>
                  ) : (
                    <div className="space-y-2">
                      {devices.map((dev) => (
                        <div key={dev.id} className="flex items-center justify-between bg-surface-container-highest rounded-lg px-4 py-2.5 border border-outline-variant/15">
                          <div className="flex items-center gap-3 min-w-0">
                            <span className={`w-2 h-2 rounded-full shrink-0 ${dev.status === 'up' ? 'bg-primary' : dev.status === 'down' ? 'bg-error' : 'bg-warning'}`} />
                            <div className="min-w-0">
                              <p className="text-sm font-bold text-on-surface truncate">{dev.name}</p>
                              <p className="text-[10px] text-on-surface-variant font-mono">{dev.ipAddress}</p>
                            </div>
                          </div>
                          <span className="text-[10px] font-bold uppercase tracking-wide text-on-surface-variant shrink-0 ml-3">{dev.protocol}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center min-h-[400px] text-on-surface-variant">
              <span className="material-symbols-outlined text-5xl mb-3 text-outline">edit_location_alt</span>
              <p className="font-headline font-bold">Select or create a location</p>
              <p className="text-xs mt-1">Choose from the tree or click "Add Location" to get started.</p>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
