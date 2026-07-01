import { useEffect, useMemo, useState } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import { createPhase2, getPhase2Summary, listPhase2, type Phase2Row, type Phase2Summary } from '../api/phase2';
import { useToast } from '../components/ui/useToast';

export interface Phase2PageConfig {
  title: string;
  subtitle: string;
  icon: string;
  path: string;
  primaryFields: string[];
  quickCreate?: Phase2Row;
}

function formatKey(key: string) {
  return key.replace(/_/g, ' ').replace(/\b\w/g, (m) => m.toUpperCase());
}

function valueText(value: unknown) {
  if (value == null) return '-';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

const summaryCards: Array<[keyof Phase2Summary, string, string]> = [
  ['locations', 'Locations', 'apartment'],
  ['contacts', 'Contacts', 'contacts'],
  ['incidents', 'Incidents', 'crisis_alert'],
  ['maintenanceWindows', 'Maintenance', 'event_repeat'],
  ['statusServices', 'Status Services', 'public'],
  ['discoveryJobs', 'Discovery Jobs', 'travel_explore'],
  ['ispLinks', 'ISP Links', 'router'],
  ['scheduledReports', 'Schedules', 'summarize'],
];

export default function Phase2Page({ config }: { config: Phase2PageConfig }) {
  const { addToast } = useToast();
  const [rows, setRows] = useState<Phase2Row[]>([]);
  const [summary, setSummary] = useState<Phase2Summary | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [creating, setCreating] = useState(false);

  const loadData = async () => {
    setLoading(true);
    try {
      const [items, counts] = await Promise.all([
        listPhase2(config.path),
        getPhase2Summary().catch(() => ({ data: null })),
      ]);
      setRows(items.data || []);
      setSummary(counts.data);
    } catch {
      setRows([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    let active = true;
    (async () => {
      setLoading(true);
      try {
        const [items, counts] = await Promise.all([
          listPhase2(config.path),
          getPhase2Summary().catch(() => ({ data: null })),
        ]);
        if (active) {
          setRows(items.data || []);
          setSummary(counts.data);
          setLoading(false);
        }
      } catch {
        if (active) {
          setRows([]);
          setLoading(false);
        }
      }
    })();
    return () => { active = false; };
  }, [config.path]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return rows;
    return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(needle));
  }, [rows, search]);

  const handleQuickCreate = async () => {
    if (!config.quickCreate) return;
    setCreating(true);
    try {
      await createPhase2(config.path, config.quickCreate);
      addToast('Created starter record', 'success');
      await loadData();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Create failed', 'error');
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="space-y-8">
      <SectionHeader
        title={config.title}
        subtitle={config.subtitle}
        action={config.quickCreate && (
          <Button icon="add" onClick={handleQuickCreate} disabled={creating}>
            Quick Add
          </Button>
        )}
      />

      {summary && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          {summaryCards.map(([key, label, icon]) => (
            <Card key={key} variant="low" className="p-4">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs uppercase tracking-wide text-on-surface-variant">{label}</span>
                <span className="material-symbols-outlined text-[18px] text-primary">{icon}</span>
              </div>
              <div className="font-headline text-2xl font-semibold mt-2">{summary[key] ?? 0}</div>
            </Card>
          ))}
        </div>
      )}

      <Card variant="low" className="overflow-hidden">
        <div className="p-5 border-b border-outline-variant/20 flex flex-col md:flex-row gap-4 md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-primary/10 text-primary flex items-center justify-center">
              <span className="material-symbols-outlined">{config.icon}</span>
            </div>
            <div>
              <h2 className="font-headline font-semibold text-lg">{config.title}</h2>
              <p className="text-xs text-on-surface-variant uppercase tracking-wide">{filtered.length} records</p>
            </div>
          </div>
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search records..."
            className="bg-surface-container-lowest border border-outline-variant/20 rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-outline focus:ring-1 focus:ring-primary outline-none md:w-72"
          />
        </div>

        {loading ? (
          <div className="p-8 text-sm text-on-surface-variant">Loading...</div>
        ) : filtered.length === 0 ? (
          <div className="p-10 text-center">
            <span className="material-symbols-outlined text-4xl text-outline">{config.icon}</span>
            <h3 className="font-headline font-semibold text-lg mt-3">No records yet</h3>
            <p className="text-sm text-on-surface-variant mt-1">Create records through the API or use Quick Add where available.</p>
          </div>
        ) : (
          <div className="divide-y divide-outline-variant/20">
            {filtered.map((row, index) => (
              <article key={valueText(row.id) + index} className="p-5 hover:bg-surface-container-low/50 transition-colors">
                <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
                  <div>
                    <h3 className="font-headline font-semibold text-lg">
                      {valueText(row.name || row.title || row.ip_address || row.subnet || `Record ${index + 1}`)}
                    </h3>
                    <p className="text-xs text-on-surface-variant uppercase tracking-wide mt-1">
                      ID {valueText(row.id)} {row.status ? ` / ${valueText(row.status)}` : ''}
                    </p>
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3 lg:max-w-3xl">
                    {config.primaryFields.map((field) => (
                      <div key={field} className="min-w-0">
                        <div className="text-[10px] text-on-surface-variant uppercase tracking-wide">{formatKey(field)}</div>
                        <div className="text-sm font-medium truncate">{valueText(row[field])}</div>
                      </div>
                    ))}
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
