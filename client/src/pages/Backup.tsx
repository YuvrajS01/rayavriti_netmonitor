import { useState, useEffect, useRef, useCallback } from 'react';
import { v1, wrap } from '../api/http';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

interface Backup {
  id: number;
  filename: string;
  size: number;
  status: 'pending' | 'running' | 'completed' | 'failed';
  type: 'manual' | 'scheduled';
  createdAt: string;
  completedAt?: string;
  error?: string;
  checksum?: string;
  restoredAt?: string;
  restoredBy?: string;
}

interface BackupConfig {
  backupDir: string;
  maxBackups: number;
  retentionDays: number;
  scheduleEnabled: boolean;
  scheduleCron: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('en-IN', {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

function statusColor(status: Backup['status']): string {
  switch (status) {
    case 'completed': return 'text-green-400';
    case 'running': return 'text-amber-400';
    case 'pending': return 'text-blue-400';
    case 'failed': return 'text-error';
    default: return 'text-on-surface-variant';
  }
}

function statusIcon(status: Backup['status']): string {
  switch (status) {
    case 'completed': return 'check_circle';
    case 'running': return 'sync';
    case 'pending': return 'schedule';
    case 'failed': return 'error';
    default: return 'help';
  }
}

export default function BackupPage() {
  const { addToast } = useToast();
  const [backups, setBackups] = useState<Backup[]>([]);
  const [config, setConfig] = useState<BackupConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [confirmRestore, setConfirmRestore] = useState<Backup | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Backup | null>(null);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchBackups = useCallback(async () => {
    try {
      const res = await v1.get('/backups');
      const result = wrap<Backup[]>(res.data);
      setBackups(result.data);
    } catch {
      addToast('Failed to load backups', 'error');
    }
  }, [addToast]);

  const fetchConfig = useCallback(async () => {
    try {
      const res = await v1.get('/backups/config');
      const result = wrap<BackupConfig>(res.data);
      setConfig(result.data);
    } catch {
      // Config endpoint may fail if backup module isn't initialized
    }
  }, []);

  // Initial data load
  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      await Promise.all([fetchBackups(), fetchConfig()]);
      if (!cancelled) setLoading(false);
    }
    load();
    return () => { cancelled = true; };
  }, [fetchBackups, fetchConfig]);

  // Poll for running backups
  useEffect(() => {
    const hasRunning = backups.some(b => b.status === 'running' || b.status === 'pending');
    if (hasRunning && !pollRef.current) {
      pollRef.current = setInterval(fetchBackups, 3000);
    } else if (!hasRunning && pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [backups, fetchBackups]);

  const handleCreateBackup = async () => {
    setCreating(true);
    try {
      await v1.post('/backups');
      addToast('Backup started', 'success');
      await fetchBackups();
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: { message?: string } } } };
      addToast(axiosErr?.response?.data?.error?.message || 'Failed to create backup', 'error');
    } finally {
      setCreating(false);
    }
  };

  const handleDownload = (backup: Backup) => {
    const url = `/api/v1/backups/${backup.id}/download`;
    window.open(url, '_blank');
  };

  const handleRestore = async () => {
    if (!confirmRestore) return;
    try {
      await v1.post(`/backups/${confirmRestore.id}/restore`);
      addToast('Database restored successfully', 'success');
      setConfirmRestore(null);
      await fetchBackups();
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: { message?: string } } } };
      addToast(axiosErr?.response?.data?.error?.message || 'Restore failed', 'error');
    }
  };

  const handleDelete = async () => {
    if (!confirmDelete) return;
    try {
      await v1.delete(`/backups/${confirmDelete.id}`);
      addToast('Backup deleted', 'success');
      setConfirmDelete(null);
      await fetchBackups();
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: { message?: string } } } };
      addToast(axiosErr?.response?.data?.error?.message || 'Failed to delete backup', 'error');
    }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const formData = new FormData();
      formData.append('file', file);
      await v1.post('/backups/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      addToast('Upload and restore completed', 'success');
      await fetchBackups();
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: { message?: string } } } };
      addToast(axiosErr?.response?.data?.error?.message || 'Upload failed', 'error');
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const totalBackups = backups.length;
  const completedBackups = backups.filter(b => b.status === 'completed').length;
  const totalSize = backups.reduce((sum, b) => sum + (b.size || 0), 0);

  return (
    <div>
      <SectionHeader
        title="Backup & Restore"
        subtitle="Create database backups, restore from previous backups, or upload a backup file."
      />

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <StatCard label="Total Backups" value={totalBackups} icon="storage" />
        <StatCard label="Completed" value={completedBackups} color="text-green-400" icon="check_circle" />
        <StatCard label="Total Size" value={formatBytes(totalSize)} icon="scale" />
        <StatCard label="Retention" value={config ? `${config.retentionDays} days` : '—'} icon="schedule" />
      </div>

      {/* Actions */}
      <div className="flex flex-wrap gap-3 mb-8">
        <Button
          variant="primary"
          icon="backup"
          onClick={handleCreateBackup}
          disabled={creating || backups.some(b => b.status === 'running' || b.status === 'pending')}
        >
          {creating ? 'Creating...' : 'Create Backup'}
        </Button>
        <Button
          variant="secondary"
          icon="upload"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading}
        >
          {uploading ? 'Uploading...' : 'Upload & Restore'}
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          accept=".sql,.gz,.backup"
          className="hidden"
          onChange={handleUpload}
        />
      </div>

      {/* Backup List */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <span className="material-symbols-outlined text-3xl text-primary animate-pulse">hourglass_top</span>
        </div>
      ) : backups.length === 0 ? (
        <EmptyState
          icon="backup"
          title="No backups yet"
          description="Create your first backup to protect your configuration and data."
        />
      ) : (
        <div className="bg-surface-container-low rounded-lg border border-outline-variant/20 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-outline-variant/20">
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">File</th>
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Type</th>
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Size</th>
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Status</th>
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Created</th>
                  <th className="text-left py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Restored</th>
                  <th className="text-right py-3 px-4 font-label text-xs text-on-surface-variant uppercase tracking-wide">Actions</th>
                </tr>
              </thead>
              <tbody>
                {backups.map(backup => (
                  <tr key={backup.id} className="border-b border-outline-variant/10 hover:bg-surface-container-lowest transition-colors">
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-2">
                        <span className="material-symbols-outlined text-lg text-on-surface-variant">description</span>
                        <span className="font-mono text-xs text-on-surface">{backup.filename}</span>
                      </div>
                      {backup.checksum && (
                        <p className="text-[10px] text-on-surface-variant mt-0.5 font-mono truncate max-w-[300px]">
                          SHA256: {backup.checksum}
                        </p>
                      )}
                      {backup.error && (
                        <p className="text-[10px] text-error mt-0.5 truncate max-w-[300px]">{backup.error}</p>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      <span className={`inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full ${
                        backup.type === 'manual'
                          ? 'bg-primary/10 text-primary'
                          : 'bg-purple-500/10 text-purple-400'
                      }`}>
                        <span className="material-symbols-outlined text-xs">
                          {backup.type === 'manual' ? 'touch_app' : 'schedule'}
                        </span>
                        {backup.type}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-on-surface-variant text-xs">
                      {formatBytes(backup.size)}
                    </td>
                    <td className="py-3 px-4">
                      <span className={`inline-flex items-center gap-1 text-xs font-medium ${statusColor(backup.status)}`}>
                        <span className="material-symbols-outlined text-sm">{statusIcon(backup.status)}</span>
                        {backup.status}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-on-surface-variant text-xs">
                      {formatDate(backup.createdAt)}
                    </td>
                    <td className="py-3 px-4 text-on-surface-variant text-xs">
                      {backup.restoredAt ? (
                        <div>
                          <span>{formatDate(backup.restoredAt)}</span>
                          {backup.restoredBy && (
                            <span className="text-on-surface-variant/60 block">by {backup.restoredBy}</span>
                          )}
                        </div>
                      ) : (
                        <span className="text-on-surface-variant/40">—</span>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      <div className="flex items-center justify-end gap-1">
                        {backup.status === 'completed' && (
                          <>
                            <button
                              onClick={() => handleDownload(backup)}
                              className="p-1.5 rounded-md text-on-surface-variant hover:text-primary hover:bg-surface-container transition-colors"
                              title="Download"
                            >
                              <span className="material-symbols-outlined text-lg">download</span>
                            </button>
                            <button
                              onClick={() => setConfirmRestore(backup)}
                              className="p-1.5 rounded-md text-on-surface-variant hover:text-amber-400 hover:bg-amber-500/10 transition-colors"
                              title="Restore from this backup"
                            >
                              <span className="material-symbols-outlined text-lg">restore</span>
                            </button>
                          </>
                        )}
                        <button
                          onClick={() => setConfirmDelete(backup)}
                          className="p-1.5 rounded-md text-on-surface-variant hover:text-error hover:bg-error/10 transition-colors"
                          title="Delete"
                        >
                          <span className="material-symbols-outlined text-lg">delete</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Backup Config Info */}
      {config && (
        <div className="mt-8 bg-surface-container-low rounded-lg p-6 border border-outline-variant/20">
          <h3 className="font-headline text-sm font-semibold text-on-surface-variant uppercase tracking-wide mb-4 flex items-center gap-2">
            <span className="material-symbols-outlined text-lg">settings</span>
            Backup Configuration
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div>
              <p className="text-xs text-on-surface-variant/60 uppercase tracking-wide mb-1">Backup Directory</p>
              <p className="font-mono text-on-surface">{config.backupDir}</p>
            </div>
            <div>
              <p className="text-xs text-on-surface-variant/60 uppercase tracking-wide mb-1">Max Backups</p>
              <p className="text-on-surface">{config.maxBackups}</p>
            </div>
            <div>
              <p className="text-xs text-on-surface-variant/60 uppercase tracking-wide mb-1">Retention</p>
              <p className="text-on-surface">{config.retentionDays} days</p>
            </div>
          </div>
        </div>
      )}

      {/* Restore Confirmation */}
      <ConfirmDialog
        open={!!confirmRestore}
        title="Restore Database"
        message={`This will overwrite the current database with the backup "${confirmRestore?.filename}". This action cannot be undone. All current data will be replaced.`}
        confirmLabel="Restore Now"
        danger
        onConfirm={handleRestore}
        onCancel={() => setConfirmRestore(null)}
      />

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={!!confirmDelete}
        title="Delete Backup"
        message={`Permanently delete backup "${confirmDelete?.filename}"? This cannot be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(null)}
      />
    </div>
  );
}
