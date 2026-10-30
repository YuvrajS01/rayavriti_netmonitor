import { v1, wrap, type RequestOptions } from './http';

export interface Backup {
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

export interface BackupConfig {
  backupDir: string;
  maxBackups: number;
  retentionDays: number;
  scheduleEnabled: boolean;
  scheduleCron: string;
}

export const getBackups = ({ signal }: RequestOptions = {}) =>
  v1.get('/backups', { signal }).then((r) => wrap<Backup[]>(r.data));

export const getBackupConfig = ({ signal }: RequestOptions = {}) =>
  v1.get('/backups/config', { signal }).then((r) => wrap<BackupConfig>(r.data));

export const createBackup = ({ signal }: RequestOptions = {}) =>
  v1.post('/backups', undefined, { signal }).then((r) => wrap<Backup>(r.data));

export const restoreBackup = (id: number, { signal }: RequestOptions = {}) =>
  v1.post(`/backups/${id}/restore`, undefined, { signal }).then((r) => r.data as unknown);

export const deleteBackup = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/backups/${id}`, { signal }).then((r) => r.data as unknown);

export const uploadBackup = (file: File, { signal }: RequestOptions = {}) => {
  const formData = new FormData();
  formData.append('file', file);
  return v1.post('/backups/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    signal,
  }).then((r) => r.data as unknown);
};

/** Absolute URL for a backup download, resolved against the configured API base. */
export const backupDownloadUrl = (id: number) =>
  `${v1.defaults.baseURL ?? '/api/v1'}/backups/${id}/download`;
