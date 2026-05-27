/**
 * Automated data retention scheduler.
 *
 * Periodically prunes old data to keep the database at a manageable size.
 * Retention periods are configurable via environment variables.
 *
 * Default retention:
 *   - Metrics:          30 days
 *   - Flow records:      7 days
 *   - Resolved alerts:  90 days
 */

import db from './database';
import logger from './logger';

const METRICS_RETENTION_DAYS = Math.max(1, Number(process.env.METRICS_RETENTION_DAYS || 30));
const FLOW_RETENTION_DAYS = Math.max(1, Number(process.env.FLOW_RETENTION_DAYS || 7));
const ALERTS_RETENTION_DAYS = Math.max(1, Number(process.env.ALERTS_RETENTION_DAYS || 90));

/** Batch size for DELETE operations to avoid locking the DB for too long. */
const BATCH_SIZE = 1000;

/** Run interval: every 6 hours. */
const RUN_INTERVAL_MS = 6 * 60 * 60 * 1000;

let retentionInterval: NodeJS.Timeout | null = null;

function cutoffDate(days: number): string {
  const d = new Date(Date.now() - days * 24 * 60 * 60 * 1000);
  return d.toISOString().slice(0, 19).replace('T', ' ');
}

function pruneTable(
  table: string,
  timestampCol: string,
  cutoff: string,
  extraWhere?: string,
): number {
  let totalDeleted = 0;

  // Delete in batches to avoid long-running locks
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const whereClause = extraWhere
      ? `${timestampCol} < ? AND ${extraWhere}`
      : `${timestampCol} < ?`;

    const result = db.raw(
      `DELETE FROM ${table} WHERE rowid IN (
        SELECT rowid FROM ${table} WHERE ${whereClause} LIMIT ?
      )`,
      cutoff,
      BATCH_SIZE,
    );

    const deleted = result?.changes ?? 0;
    totalDeleted += deleted;

    if (deleted < BATCH_SIZE) {
      break;
    }
  }

  return totalDeleted;
}

function runRetention(): void {
  const log = logger.child({ component: 'retention' });

  log.info('Starting data retention sweep');

  try {
    const metricsCutoff = cutoffDate(METRICS_RETENTION_DAYS);
    const metricsDeleted = pruneTable('metrics', 'timestamp', metricsCutoff);
    if (metricsDeleted > 0) {
      log.info({ table: 'metrics', deleted: metricsDeleted, cutoff: metricsCutoff },
        `Pruned ${metricsDeleted} metrics older than ${METRICS_RETENTION_DAYS} days`);
    }
  } catch (err) {
    log.error({ err, table: 'metrics' }, 'Failed to prune metrics');
  }

  try {
    const flowsCutoff = cutoffDate(FLOW_RETENTION_DAYS);
    const flowsDeleted = pruneTable('flow_records', 'timestamp', flowsCutoff);
    if (flowsDeleted > 0) {
      log.info({ table: 'flow_records', deleted: flowsDeleted, cutoff: flowsCutoff },
        `Pruned ${flowsDeleted} flow records older than ${FLOW_RETENTION_DAYS} days`);
    }
  } catch (err) {
    log.error({ err, table: 'flow_records' }, 'Failed to prune flow records');
  }

  try {
    const alertsCutoff = cutoffDate(ALERTS_RETENTION_DAYS);
    const alertsDeleted = pruneTable(
      'alerts',
      'created_at',
      alertsCutoff,
      "status = 'resolved'",
    );
    if (alertsDeleted > 0) {
      log.info({ table: 'alerts', deleted: alertsDeleted, cutoff: alertsCutoff },
        `Pruned ${alertsDeleted} resolved alerts older than ${ALERTS_RETENTION_DAYS} days`);
    }
  } catch (err) {
    log.error({ err, table: 'alerts' }, 'Failed to prune resolved alerts');
  }

  // Also prune old stopped capture sessions (keep 30 days)
  try {
    const sessionsCutoff = cutoffDate(30);
    const sessionsDeleted = pruneTable(
      'capture_sessions',
      'started_at',
      sessionsCutoff,
      "status != 'running'",
    );
    if (sessionsDeleted > 0) {
      log.info({ table: 'capture_sessions', deleted: sessionsDeleted },
        `Pruned ${sessionsDeleted} old capture sessions`);
    }
  } catch (err) {
    log.error({ err, table: 'capture_sessions' }, 'Failed to prune capture sessions');
  }

  log.info('Data retention sweep complete');
}

export function startRetentionScheduler(): void {
  if (retentionInterval) {
    clearInterval(retentionInterval);
  }

  // Run once at startup (delayed 30 seconds to let the server finish booting)
  setTimeout(() => {
    runRetention();
  }, 30_000);

  retentionInterval = setInterval(runRetention, RUN_INTERVAL_MS);

  logger.info(
    {
      metricsRetentionDays: METRICS_RETENTION_DAYS,
      flowRetentionDays: FLOW_RETENTION_DAYS,
      alertsRetentionDays: ALERTS_RETENTION_DAYS,
      intervalHours: RUN_INTERVAL_MS / (60 * 60 * 1000),
    },
    'Data retention scheduler started',
  );
}

export function stopRetentionScheduler(): void {
  if (retentionInterval) {
    clearInterval(retentionInterval);
    retentionInterval = null;
  }
}

