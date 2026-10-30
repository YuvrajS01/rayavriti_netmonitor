import { useState, useEffect, useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { backupDownloadUrl, createBackup, deleteBackup, getBackupConfig, getBackups, restoreBackup, uploadBackup, type Backup, type BackupConfig } from '../api/backups';
import { queryKeys } from '../api/queryKeys';
import SectionHeader from '../components/ui/SectionHeader';
import StatCard from '../components/ui/StatCard';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { useToast } from '../components/ui/useToast';

const EMPTY_BACKUPS: Backup[] = [];

function apiErrorMessage(err: unknown, fallback: string): string {
  const axiosErr = err as { response?: { data?: { error?: { message?: string } } } };
  return axiosErr?.response?.data?.error?.message || fallback;
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
    default: return 'text-fg-muted';
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
  const queryClient = useQueryClient();
  const [confirmRestore, setConfirmRestore] = useState<Backup | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Backup | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Poll every 3s only while a backup is pending/running (and the tab is visible).
  const backupsQuery = useQuery({
    queryKey: queryKeys.backups.list(),
    queryFn: ({ signal }) => getBackups({ signal }),
    refetchInterval: (query) =>
      query.state.data?.data?.some((b) => b.status === 'running' || b.status === 'pending') ? 3000 : false,
  });
  // The config endpoint may fail if the backup module isn't initialized.
  const configQuery = useQuery({
    queryKey: queryKeys.backups.config(),
    queryFn: ({ signal }) => getBackupConfig({ signal }),
    retry: false,
  });
  const loading = backupsQuery.isPending || configQuery.isPending;
  const backups = backupsQuery.data?.data || EMPTY_BACKUPS;
  const config: BackupConfig | null = configQuery.data?.data ?? null;

  const loadFailedAt = backupsQuery.errorUpdatedAt;
  useEffect(() => {
    if (loadFailedAt) addToast('Failed to load backups', 'error');
  }, [loadFailedAt, addToast]);

  const refreshBackups = () => queryClient.invalidateQueries({ queryKey: queryKeys.backups.list() });

  const createMutation = useMutation({
    mutationFn: () => createBackup(),
    onSuccess: () => {
      addToast('Backup started', 'success');
      return refreshBackups();
    },
    onError: (err) => addToast(apiErrorMessage(err, 'Failed to create backup'), 'error'),
  });
  const creating = createMutation.isPending;

  const restoreMutation = useMutation({
    mutationFn: (backup: Backup) => restoreBackup(backup.id),
    onSuccess: () => {
      addToast('Database restored successfully', 'success');
      setConfirmRestore(null);
      return refreshBackups();
    },
    onError: (err) => addToast(apiErrorMessage(err, 'Restore failed'), 'error'),
  });

  const deleteMutation = useMutation({
    mutationFn: (backup: Backup) => deleteBackup(backup.id),
    onSuccess: () => {
      addToast('Backup deleted', 'success');
      setConfirmDelete(null);
      return refreshBackups();
    },
    onError: (err) => addToast(apiErrorMessage(err, 'Failed to delete backup'), 'error'),
  });

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadBackup(file),
    onSuccess: () => {
      addToast('Upload and restore completed', 'success');
      return refreshBackups();
    },
    onError: (err) => addToast(apiErrorMessage(err, 'Upload failed'), 'error'),
    onSettled: () => {
      if (fileInputRef.current) fileInputRef.current.value = '';
    },
  });
  const uploading = uploadMutation.isPending;

  const handleCreateBackup = () => createMutation.mutate();

  const handleDownload = (backup: Backup) => {
    window.open(backupDownloadUrl(backup.id), '_blank', 'noopener,noreferrer');
  };

  const handleRestore = () => {
    if (confirmRestore) restoreMutation.mutate(confirmRestore);
  };

  const handleDelete = () => {
    if (confirmDelete) deleteMutation.mutate(confirmDelete);
  };

  const handleUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) uploadMutation.mutate(file);
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
        <div className="bg-surface-1 rounded-lg border border-line overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-line">
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">File</th>
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Type</th>
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Size</th>
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Status</th>
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Created</th>
                  <th className="text-left py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Restored</th>
                  <th className="text-right py-3 px-4 font-sans text-xs text-fg-muted uppercase tracking-wide">Actions</th>
                </tr>
              </thead>
              <tbody>
                {backups.map(backup => (
                  <tr key={backup.id} className="border-b border-line hover:bg-inset transition-colors">
                    <td className="py-3 px-4">
                      <div className="flex items-center gap-2">
                        <span className="material-symbols-outlined text-lg text-fg-muted">description</span>
                        <span className="font-mono text-xs text-fg">{backup.filename}</span>
                      </div>
                      {backup.checksum && (
                        <p className="text-[10px] text-fg-muted mt-0.5 font-mono truncate max-w-[300px]">
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
                    <td className="py-3 px-4 text-fg-muted text-xs">
                      {formatBytes(backup.size)}
                    </td>
                    <td className="py-3 px-4">
                      <span className={`inline-flex items-center gap-1 text-xs font-medium ${statusColor(backup.status)}`}>
                        <span className="material-symbols-outlined text-sm">{statusIcon(backup.status)}</span>
                        {backup.status}
                      </span>
                    </td>
                    <td className="py-3 px-4 text-fg-muted text-xs">
                      {formatDate(backup.createdAt)}
                    </td>
                    <td className="py-3 px-4 text-fg-muted text-xs">
                      {backup.restoredAt ? (
                        <div>
                          <span>{formatDate(backup.restoredAt)}</span>
                          {backup.restoredBy && (
                            <span className="text-fg-muted/60 block">by {backup.restoredBy}</span>
                          )}
                        </div>
                      ) : (
                        <span className="text-fg-muted/40">—</span>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      <div className="flex items-center justify-end gap-1">
                        {backup.status === 'completed' && (
                          <>
                            <button
                              onClick={() => handleDownload(backup)}
                              className="p-1.5 rounded-md text-fg-muted hover:text-primary hover:bg-surface-2 transition-colors"
                              title="Download"
                            >
                              <span className="material-symbols-outlined text-lg">download</span>
                            </button>
                            <button
                              onClick={() => setConfirmRestore(backup)}
                              className="p-1.5 rounded-md text-fg-muted hover:text-amber-400 hover:bg-amber-500/10 transition-colors"
                              title="Restore from this backup"
                            >
                              <span className="material-symbols-outlined text-lg">restore</span>
                            </button>
                          </>
                        )}
                        <button
                          onClick={() => setConfirmDelete(backup)}
                          className="p-1.5 rounded-md text-fg-muted hover:text-error hover:bg-error/10 transition-colors"
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
        <div className="mt-8 bg-surface-1 rounded-lg p-6 border border-line">
          <h3 className="font-sans text-sm font-semibold text-fg-muted uppercase tracking-wide mb-4 flex items-center gap-2">
            <span className="material-symbols-outlined text-lg">settings</span>
            Backup Configuration
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
            <div>
              <p className="text-xs text-fg-muted/60 uppercase tracking-wide mb-1">Backup Directory</p>
              <p className="font-mono text-fg">{config.backupDir}</p>
            </div>
            <div>
              <p className="text-xs text-fg-muted/60 uppercase tracking-wide mb-1">Max Backups</p>
              <p className="text-fg">{config.maxBackups}</p>
            </div>
            <div>
              <p className="text-xs text-fg-muted/60 uppercase tracking-wide mb-1">Retention</p>
              <p className="text-fg">{config.retentionDays} days</p>
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
