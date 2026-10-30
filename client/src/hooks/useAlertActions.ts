import { useCallback } from 'react';
import { useMutation, useQueryClient, type QueryClient, type QueryKey } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { acknowledgeAlert, bulkAlertAction, resolveAlert, ALERT_BULK_MAX } from '../api/alerts';
import { patchAlertLists } from '../api/liveCache';
import { queryKeys } from '../api/queryKeys';
import type { Alert, AlertBulkAction, AlertBulkResult } from '../api/types';
import { useToast } from '../components/ui/useToast';
import { useSessionUser } from '../store/session';

type Snapshot = Array<[QueryKey, unknown]>;

const NEXT_STATUS: Record<AlertBulkAction, Alert['status']> = { acknowledge: 'acknowledged', resolve: 'resolved' };
const PAST: Record<AlertBulkAction, string> = { acknowledge: 'Acknowledged', resolve: 'Resolved' };

function snapshotLists(client: QueryClient): Snapshot {
  return client.getQueriesData({ queryKey: [...queryKeys.alerts.all, 'list'] });
}

function restore(client: QueryClient, snapshot: Snapshot | undefined) {
  for (const [key, value] of snapshot ?? []) client.setQueryData(key, value);
}

/** Apply the expected result to every cached list right away. */
function optimistic(client: QueryClient, ids: number[], action: AlertBulkAction, by?: string): Snapshot {
  const snapshot = snapshotLists(client);
  const now = new Date().toISOString();
  for (const id of ids) {
    patchAlertLists(client, {
      id,
      status: NEXT_STATUS[action],
      ...(action === 'acknowledge' ? { acknowledgedAt: now, acknowledgedBy: by } : { resolvedAt: now, resolvedBy: by }),
    });
  }
  return snapshot;
}

/**
 * v5.0 servers have no /alerts/bulk; fall back to one call per alert so the
 * workbench still works against them.
 */
async function bulkWithFallback(ids: number[], action: AlertBulkAction, note?: string): Promise<AlertBulkResult> {
  try {
    return (await bulkAlertAction(ids, action, note)).data;
  } catch (error) {
    const status = isAxiosError(error) ? error.response?.status : undefined;
    if (status !== 404 && status !== 405) throw error;
    const call = action === 'acknowledge' ? acknowledgeAlert : resolveAlert;
    const results = await Promise.allSettled(ids.map((id) => call(id)));
    return {
      updated: ids.filter((_, i) => results[i]!.status === 'fulfilled'),
      skipped: ids
        .filter((_, i) => results[i]!.status === 'rejected')
        .map((id) => ({ id, reason: 'invalid_state' as const })),
    };
  }
}

/**
 * Acknowledge/resolve with optimistic cache updates: lists change at once,
 * roll back if the server refuses, and reconcile with a refetch after.
 */
export function useAlertActions() {
  const client = useQueryClient();
  const { addToast } = useToast();
  const user = useSessionUser();

  const settle = () => client.invalidateQueries({ queryKey: queryKeys.alerts.all });

  const single = useMutation({
    mutationFn: ({ id, action }: { id: number; action: AlertBulkAction }) => (action === 'acknowledge' ? acknowledgeAlert(id) : resolveAlert(id)),
    onMutate: ({ id, action }) => ({ snapshot: optimistic(client, [id], action, user?.username) }),
    onError: (_e, { action }, ctx) => {
      restore(client, ctx?.snapshot);
      addToast(`Could not ${action} the alert. It has been put back.`, 'error');
    },
    onSettled: settle,
  });

  const bulk = useMutation({
    mutationFn: ({ ids, action, note }: { ids: number[]; action: AlertBulkAction; note?: string }) =>
      bulkWithFallback(ids.slice(0, ALERT_BULK_MAX), action, note),
    onMutate: ({ ids, action }) => ({ snapshot: optimistic(client, ids, action, user?.username) }),
    onSuccess: (result, { action, ids }) => {
      const skipped = result.skipped.length;
      const dropped = Math.max(0, ids.length - ALERT_BULK_MAX);
      addToast(
        `${PAST[action]} ${result.updated.length} alert${result.updated.length === 1 ? '' : 's'}` +
          (skipped ? `; ${skipped} skipped` : '') +
          (dropped ? `; ${dropped} over the ${ALERT_BULK_MAX} limit were not sent` : ''),
        skipped ? 'info' : 'success',
      );
    },
    onError: (_e, { action }, ctx) => {
      restore(client, ctx?.snapshot);
      addToast(`Bulk ${action} failed. Nothing was changed.`, 'error');
    },
    onSettled: settle,
  });

  const { mutate: mutateSingle } = single;
  const { mutateAsync: mutateBulk } = bulk;
  const act = useCallback((id: number, action: AlertBulkAction) => mutateSingle({ id, action }), [mutateSingle]);
  const bulkAct = useCallback((ids: number[], action: AlertBulkAction, note?: string) => mutateBulk({ ids, action, note }), [mutateBulk]);
  return {
    act,
    bulkAct,
    pending: single.isPending || bulk.isPending,
  };
}
