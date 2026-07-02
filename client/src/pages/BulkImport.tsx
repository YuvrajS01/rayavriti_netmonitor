import { useState, useMemo, useCallback } from 'react';
import SectionHeader from '../components/ui/SectionHeader';
import Card from '../components/ui/Card';
import Button from '../components/ui/Button';
import { createPhase2 } from '../api/phase2';
import { v1 } from '../api/client';
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
  const headers = lines[0].split(',').map((h) => h.trim().replace(/"/g, ''));
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
          await createPhase2('/import/devices', {
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
      const res = await v1.get('/import/template', { responseType: 'blob' });
      const blob = new Blob([res.data as BlobPart]);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'device_import_template.csv';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      addToast('Failed to download template', 'error');
    }
  }, [addToast]);

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Bulk Device Import"
        subtitle="Upload a CSV file to import multiple devices at once. Download the template for the expected format."
        action={
          <Button variant="secondary" icon="download" onClick={downloadTemplate}>
            Download Template
          </Button>
        }
      />

      {/* Step indicator */}
      <div className="flex items-center gap-2 text-xs">
        {['Upload', 'Preview', 'Result'].map((s, i) => {
          const stepKeys: Step[] = ['upload', 'preview', 'result'];
          const isActive = step === stepKeys[i];
          const isDone = stepKeys.indexOf(step) > i;
          return (
            <div key={s} className="flex items-center gap-2">
              {i > 0 && <span className="w-8 h-px bg-outline-variant/30" />}
              <span
                className={[
                  'w-7 h-7 rounded-full flex items-center justify-center font-semibold text-xs transition-colors',
                  isActive ? 'bg-primary text-on-primary' : isDone ? 'bg-primary/20 text-primary' : 'bg-surface-container-lowest text-on-surface-variant',
                ].join(' ')}
              >
                {isDone ? '✓' : i + 1}
              </span>
              <span className={isActive ? 'text-primary font-semibold uppercase tracking-wide' : 'text-on-surface-variant uppercase tracking-wide'}>
                {s}
              </span>
            </div>
          );
        })}
      </div>

      {/* STEP 1: Upload */}
      {step === 'upload' && (
        <Card variant="low" className="p-8">
          <div
            onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            className={[
              'border-2 border-dashed rounded-xl p-12 text-center transition-colors duration-200 cursor-pointer',
              dragOver ? 'border-primary bg-primary/5' : 'border-outline-variant/30 hover:border-outline/50',
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
            <span className={`material-symbols-outlined text-5xl mb-4 block transition-colors ${dragOver ? 'text-primary' : 'text-outline'}`}>
              upload_file
            </span>
            <p className="font-headline font-semibold text-lg mb-2">
              {dragOver ? 'Drop your CSV file' : 'Drag & drop your CSV file'}
            </p>
            <p className="text-sm text-on-surface-variant mb-4">or click to browse</p>
            <p className="text-xs text-outline">
              Expected columns: {EXPECTED_HEADERS.join(', ')}
            </p>
          </div>
        </Card>
      )}

      {/* STEP 2: Preview */}
      {step === 'preview' && (
        <>
          {/* Summary bar */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {[
              { label: 'Total Rows', value: stats.total, color: 'text-on-surface' },
              { label: 'Valid', value: stats.valid, color: 'text-success' },
              { label: 'Warnings', value: stats.warnings, color: 'text-warning' },
              { label: 'Errors', value: stats.errors, color: 'text-error' },
              { label: 'Duplicates', value: stats.dupes, color: 'text-info' },
            ].map((s) => (
              <Card key={s.label} variant="low" className="p-3 text-center">
                <div className={`font-headline text-2xl font-semibold ${s.color}`}>{s.value}</div>
                <div className="text-[10px] uppercase tracking-wide text-on-surface-variant mt-1">{s.label}</div>
              </Card>
            ))}
          </div>

          {/* Table */}
          <Card variant="low" className="overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-outline-variant/20 bg-surface-container">
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold w-12">#</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Status</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Name</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Host</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Protocol</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Category</th>
                    <th className="px-4 py-3 text-left text-[10px] uppercase tracking-wide text-on-surface-variant font-semibold">Location</th>
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
                            onClick={() => hasIssues && setExpandedRow(isExpanded ? null : rv.lineNumber)}
                            className={[
                              'flex items-center px-4 py-2.5 transition-colors',
                              !rv.valid ? 'bg-error/5' : rv.warnings.length > 0 ? 'bg-warning/5' : '',
                              hasIssues ? 'cursor-pointer hover:bg-surface-container-low/60' : '',
                            ].join(' ')}
                          >
                            <span className="w-12 text-on-surface-variant text-xs shrink-0">{rv.lineNumber}</span>
                            <span className="w-16 shrink-0">
                              {!rv.valid ? (
                                <span className="material-symbols-outlined text-error text-base">error</span>
                              ) : rv.warnings.length > 0 ? (
                                <span className="material-symbols-outlined text-warning text-base">warning</span>
                              ) : (
                                <span className="material-symbols-outlined text-success text-base">check_circle</span>
                              )}
                            </span>
                            <span className="flex-1 min-w-0 font-medium truncate">{rv.row.name || '—'}</span>
                            <span className="flex-1 min-w-0 text-on-surface-variant font-data text-xs truncate">{rv.row.host || '—'}</span>
                            <span className="w-20 text-on-surface-variant text-xs uppercase shrink-0">{rv.row.protocol || 'ping'}</span>
                            <span className="w-24 text-on-surface-variant text-xs truncate shrink-0">{rv.row.device_category || '—'}</span>
                            <span className="w-20 text-on-surface-variant text-xs truncate shrink-0">{rv.row.location_code || '—'}</span>
                          </div>
                          {/* Expanded validation messages */}
                          {isExpanded && hasIssues && (
                            <div className="px-4 pb-3 pl-28 space-y-1">
                              {rv.errors.map((e, i) => (
                                <p key={i} className="text-xs text-error flex items-center gap-1">
                                  <span className="material-symbols-outlined text-xs">cancel</span> {e}
                                </p>
                              ))}
                              {rv.warnings.map((w, i) => (
                                <p key={i} className="text-xs text-warning flex items-center gap-1">
                                  <span className="material-symbols-outlined text-xs">warning</span> {w}
                                </p>
                              ))}
                            </div>
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
              {importing ? 'Importing...' : `Import ${stats.valid + stats.warnings} Devices`}
            </Button>
          </div>
        </>
      )}

      {/* STEP 3: Result */}
      {step === 'result' && result && (
        <Card variant="low" className="p-8 text-center">
          <span className="material-symbols-outlined text-6xl text-success mb-4 block">task_alt</span>
          <h2 className="font-headline font-semibold text-2xl mb-2">Import Complete</h2>
          <p className="text-on-surface-variant mb-6">
            <span className="text-success font-semibold">{result.created}</span> devices imported successfully
            {result.errors.length > 0 && (
              <>, <span className="text-error font-semibold">{result.errors.length}</span> failed</>
            )}
          </p>

          {result.errors.length > 0 && (
            <div className="text-left max-w-xl mx-auto mb-6">
              <h3 className="text-xs uppercase tracking-wide text-error font-semibold mb-2">Errors</h3>
              <div className="bg-error/5 rounded-lg p-3 space-y-1">
                {result.errors.slice(0, 10).map((e, i) => (
                  <p key={i} className="text-xs text-error">{e}</p>
                ))}
                {result.errors.length > 10 && (
                  <p className="text-xs text-on-surface-variant">...and {result.errors.length - 10} more</p>
                )}
              </div>
            </div>
          )}

          <Button icon="refresh" onClick={handleReset}>
            Import More
          </Button>
        </Card>
      )}
    </div>
  );
}
