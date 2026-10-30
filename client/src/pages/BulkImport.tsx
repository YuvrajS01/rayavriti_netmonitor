import { useState, useMemo, useCallback } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import { StatusPill, StatusDot } from '../components/ui/Status';
import { statusMeta } from '../utils/status';
import Button from '../components/ui/Button';
import { createResource } from '../api/resources';
import { downloadImportTemplate } from '../api/devices';
import { saveBlob } from '../api/http';
import { useToast } from '../components/ui/useToast';

// ─── CSV Parsing ────────────────────────────────────────────────

const EXPECTED_HEADERS = [
  'name', 'host', 'protocol', 'port', 'device_category',
  'location_code', 'parent_device_host', 'mac_address',
  'asset_tag', 'contact_email', 'notes',
];

type CSVRow = Record<string, string>;

function parseCSV(text: string): CSVRow[] {
  const lines = text.split('\n').filter((l) => l.trim());
  if (lines.length < 2) return [];
  const headers = (lines[0] ?? '').split(',').map((h) => h.trim().replace(/"/g, ''));
  return lines.slice(1).map((line) => {
    const values: string[] = [];
    let current = '';
    let inQuotes = false;
    for (const char of line) {
      if (char === '"') { inQuotes = !inQuotes; continue; }
      if (char === ',' && !inQuotes) { values.push(current.trim()); current = ''; continue; }
      current += char;
    }
    values.push(current.trim());
    const row: CSVRow = {};
    headers.forEach((h, i) => { row[h] = values[i] || ''; });
    return row;
  });
}

// ─── Validation ─────────────────────────────────────────────────

interface RowValidation {
  row: CSVRow;
  lineNumber: number;
  valid: boolean;
  errors: string[];
  warnings: string[];
}

const MAC_RE = /^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/;
const IP_RE = /^(\d{1,3}\.){3}\d{1,3}$/;
const HOST_RE = /^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?$/;

function validateRows(rows: CSVRow[]): RowValidation[] {
  const seen = new Set<string>();
  return rows.map((row, i) => {
    const errors: string[] = [];
    const warnings: string[] = [];

    if (!row.name?.trim()) errors.push('Name is required');
    if (!row.host?.trim()) {
      errors.push('Host is required');
    } else {
      const h = row.host.trim();
      if (!IP_RE.test(h) && !HOST_RE.test(h)) {
        warnings.push('Host does not look like a valid IP or hostname');
      }
      if (seen.has(h)) warnings.push('Duplicate host in this import');
      seen.add(h);
    }

    if (row.port && isNaN(Number(row.port))) errors.push('Port must be a number');
    if (row.port && Number(row.port) < 0) errors.push('Port must be positive');

    if (row.mac_address?.trim() && !MAC_RE.test(row.mac_address.trim())) {
      warnings.push('MAC address format should be XX:XX:XX:XX:XX:XX');
    }

    if (!row.protocol?.trim()) row.protocol = 'ping';

    return {
      row,
      lineNumber: i + 2,
      valid: errors.length === 0,
      errors,
      warnings,
    };
  });
}

type Step = 'upload' | 'preview' | 'result';

// ─── Component ──────────────────────────────────────────────────

export default function BulkImport() {
  const { addToast } = useToast();
  const [step, setStep] = useState<Step>('upload');
  const [rows, setRows] = useState<RowValidation[]>([]);
  const [expandedRow, setExpandedRow] = useState<number | null>(null);
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<{ created: number; errors: string[] } | null>(null);
  const [dragOver, setDragOver] = useState(false);

  const stats = useMemo(() => {
    const valid = rows.filter((r) => r.valid && r.warnings.length === 0).length;
    const warnings = rows.filter((r) => r.valid && r.warnings.length > 0).length;
    const errors = rows.filter((r) => !r.valid).length;
    const dupes = rows.filter((r) => r.warnings.some((w) => w.includes('Duplicate'))).length;
    return { total: rows.length, valid, warnings, errors, dupes };
  }, [rows]);

  const handleFile = useCallback((file: File) => {
    if (!file.name.endsWith('.csv')) {
      addToast('Please upload a .csv file', 'error');
      return;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      const text = e.target?.result as string;
      const parsed = parseCSV(text);
      if (parsed.length === 0) {
        addToast('No data rows found in CSV', 'error');
        return;
      }
      const validated = validateRows(parsed);
      setRows(validated);
      setStep('preview');
    };
    reader.readAsText(file);
  }, [addToast]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files[0];
    if (file) handleFile(file);
  }, [handleFile]);

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) handleFile(file);
  }, [handleFile]);

  const handleImport = async () => {
    setImporting(true);
    const validRows = rows.filter((r) => r.valid);
    const errors: string[] = [];
    let created = 0;
    try {
      for (const rv of validRows) {
        try {
          await createResource('/import/devices', {
            name: rv.row.name,
            ip_address: rv.row.host,
            protocol: rv.row.protocol || 'ping',
            port: rv.row.port ? Number(rv.row.port) : 0,
            device_category: rv.row.device_category || '',
            mac_address: rv.row.mac_address || '',
            asset_tag: rv.row.asset_tag || '',
            notes: rv.row.notes || '',
            enabled: true,
            status: 'unknown',
          });
          created++;
        } catch (err) {
          errors.push(`Row ${rv.lineNumber}: ${err instanceof Error ? err.message : 'Failed'}`);
        }
      }
      setResult({ created, errors });
      setStep('result');
      addToast(`Imported ${created} devices`, 'success');
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Import failed', 'error');
    }
    setImporting(false);
  };

  const handleReset = useCallback(() => {
    setStep('upload');
    setRows([]);
    setResult(null);
    setExpandedRow(null);
  }, []);

  const downloadTemplate = useCallback(async () => {
    try {
      saveBlob(await downloadImportTemplate(), 'device_import_template.csv');
    } catch {
      addToast('Failed to download template', 'error');
    }
  }, [addToast]);

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Bulk device import"
        subtitle="Upload a CSV file to import multiple devices at once. Download the template for the expected format."
        action={
          <Button variant="secondary" icon="download" onClick={downloadTemplate}>
            Download template
          </Button>
        }
      />

      {/* Step indicator */}
      <ol className="flex items-center gap-2 text-sm" aria-label="Import steps">
        {['Upload', 'Preview', 'Result'].map((s, i) => {
          const stepKeys: Step[] = ['upload', 'preview', 'result'];
          const isActive = step === stepKeys[i];
          const isDone = stepKeys.indexOf(step) > i;
          return (
            <li key={s} className="flex items-center gap-2" aria-current={isActive ? 'step' : undefined}>
              {i > 0 && <span className="w-8 h-px bg-line-strong" aria-hidden="true" />}
              <span
                className={[
                  'w-7 h-7 rounded-full flex items-center justify-center font-semibold text-xs transition-colors',
                  isActive ? 'bg-primary text-on-primary' : isDone ? 'bg-surface-3 text-fg' : 'bg-surface-2 text-fg-muted',
                ].join(' ')}
                aria-hidden="true"
              >
                {isDone ? '✓' : i + 1}
              </span>
              <span className={isActive ? 'text-fg font-semibold' : 'text-fg-muted'}>
                {s}{isDone && <span className="sr-only"> (done)</span>}
              </span>
            </li>
          );
        })}
      </ol>

      {/* STEP 1: Upload */}
      {step === 'upload' && (
        <Card variant="low" className="p-8">
          <div
            role="button"
            tabIndex={0}
            aria-label="Choose a CSV file to import"
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); document.getElementById('csv-input')?.click(); } }}
            onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            className={[
              'border-2 border-dashed rounded-lg p-12 text-center transition-colors duration-[var(--duration-fast)] cursor-pointer',
              dragOver ? 'border-primary bg-surface-2' : 'border-line-strong hover:border-fg-subtle hover:bg-surface-2',
            ].join(' ')}
            onClick={() => document.getElementById('csv-input')?.click()}
          >
            <input
              id="csv-input"
              type="file"
              accept=".csv"
              className="hidden"
              onChange={handleInputChange}
            />
            <span className={`material-symbols-outlined text-5xl mb-4 block transition-colors ${dragOver ? 'text-fg' : 'text-fg-subtle'}`} aria-hidden="true">
              upload_file
            </span>
            <p className="font-semibold text-lg mb-2">
              {dragOver ? 'Drop your CSV file' : 'Drag & drop your CSV file'}
            </p>
            <p className="text-sm text-fg-muted mb-4">or click to browse</p>
            <p className="text-xs text-fg-subtle">
              Expected columns: <span className="font-mono">{EXPECTED_HEADERS.join(', ')}</span>
            </p>
          </div>
        </Card>
      )}

      {/* STEP 2: Preview */}
      {step === 'preview' && (
        <>
          {/* Summary bar */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {([
              { label: 'Total rows', value: stats.total, status: null },
              { label: 'Valid', value: stats.valid, status: 'up' },
              { label: 'Warnings', value: stats.warnings, status: 'warning' },
              { label: 'Errors', value: stats.errors, status: 'down' },
              { label: 'Duplicates', value: stats.dupes, status: 'info' },
            ] as const).map((s) => (
              <Card key={s.label} variant="low" className="p-3">
                <div className="flex items-center gap-1.5 text-xs text-fg-muted">
                  {s.status && <StatusDot status={s.status} size={8} label="" />}
                  {s.label}
                </div>
                <div className="text-2xl font-semibold tabular-nums text-fg mt-1">{s.value}</div>
              </Card>
            ))}
          </div>

          {/* Table */}
          <Card variant="low" className="overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-line bg-surface-2">
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold w-12">#</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Status</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Name</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Host</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Protocol</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Category</th>
                    <th className="px-4 py-3 text-left text-xs text-fg-muted font-semibold">Location</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((rv) => {
                    const hasIssues = rv.errors.length > 0 || rv.warnings.length > 0;
                    const isExpanded = expandedRow === rv.lineNumber;
                    return (
                      <tr key={rv.lineNumber} className="group">
                        <td colSpan={7} className="p-0">
                          <div
                            role={hasIssues ? 'button' : undefined}
                            tabIndex={hasIssues ? 0 : undefined}
                            aria-expanded={hasIssues ? isExpanded : undefined}
                            onClick={() => hasIssues && setExpandedRow(isExpanded ? null : rv.lineNumber)}
                            onKeyDown={(e) => { if (hasIssues && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); setExpandedRow(isExpanded ? null : rv.lineNumber); } }}
                            className={[
                              'flex items-center px-4 py-2.5 transition-colors',
                              !rv.valid ? statusMeta('down').chip : rv.warnings.length > 0 ? statusMeta('warning').chip : '',
                              hasIssues ? 'cursor-pointer hover:bg-surface-2' : '',
                            ].join(' ')}
                          >
                            <span className="w-12 text-fg-muted font-mono text-xs shrink-0">{rv.lineNumber}</span>
                            <span className="w-28 shrink-0">
                              {!rv.valid ? (
                                <StatusPill status="down">Error</StatusPill>
                              ) : rv.warnings.length > 0 ? (
                                <StatusPill status="warning" />
                              ) : (
                                <StatusPill status="up">Valid</StatusPill>
                              )}
                            </span>
                            <span className="flex-1 min-w-0 font-medium truncate">{rv.row.name || '—'}</span>
                            <span className="flex-1 min-w-0 text-fg-muted font-mono text-xs truncate">{rv.row.host || '—'}</span>
                            <span className="w-20 text-fg-muted text-xs uppercase shrink-0">{rv.row.protocol || 'ping'}</span>
                            <span className="w-24 text-fg-muted text-xs truncate shrink-0">{rv.row.device_category || '—'}</span>
                            <span className="w-20 text-fg-muted text-xs truncate shrink-0">{rv.row.location_code || '—'}</span>
                          </div>
                          {/* Expanded validation messages */}
                          {isExpanded && hasIssues && (
                            <ul className="px-4 pb-3 pl-44 space-y-1">
                              {rv.errors.map((e, i) => (
                                <li key={i} className="text-xs text-error flex items-center gap-1.5">
                                  <span className="material-symbols-outlined text-sm" aria-hidden="true">cancel</span> {e}
                                </li>
                              ))}
                              {rv.warnings.map((w, i) => (
                                <li key={i} className="text-xs text-warning flex items-center gap-1.5">
                                  <span className="material-symbols-outlined text-sm" aria-hidden="true">warning</span> {w}
                                </li>
                              ))}
                            </ul>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </Card>

          {/* Actions */}
          <div className="flex items-center gap-3">
            <Button variant="secondary" icon="arrow_back" onClick={handleReset}>
              Back
            </Button>
            <Button
              icon="cloud_upload"
              onClick={handleImport}
              disabled={importing || stats.valid + stats.warnings === 0}
            >
              {importing ? 'Importing…' : `Import ${stats.valid + stats.warnings} devices`}
            </Button>
          </div>
        </>
      )}

      {/* STEP 3: Result */}
      {step === 'result' && result && (
        <Card variant="low" className="p-8 text-center">
          <span className="material-symbols-outlined text-6xl text-success mb-4 block" aria-hidden="true">task_alt</span>
          <h2 className="font-semibold text-2xl mb-2">Import complete</h2>
          <div className="mb-6 flex flex-wrap items-center justify-center gap-2">
            <StatusPill status="up" size="md">{result.created} imported</StatusPill>
            {result.errors.length > 0 && <StatusPill status="down" size="md">{result.errors.length} failed</StatusPill>}
          </div>

          {result.errors.length > 0 && (
            <div className="text-left max-w-xl mx-auto mb-6">
              <h3 className="text-sm text-fg font-semibold mb-2">Errors</h3>
              <div className={`${statusMeta('down').chip} rounded-md p-3 space-y-1`}>
                {result.errors.slice(0, 10).map((e, i) => (
                  <p key={i} className="text-xs text-error">{e}</p>
                ))}
                {result.errors.length > 10 && (
                  <p className="text-xs text-fg-muted">…and {result.errors.length - 10} more</p>
                )}
              </div>
            </div>
          )}

          <Button icon="refresh" onClick={handleReset}>
            Import more
          </Button>
        </Card>
      )}
    </div>
  );
}
