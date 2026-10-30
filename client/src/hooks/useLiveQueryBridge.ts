import { useQueryClient } from '@tanstack/react-query';
import { useSocketEvents } from './useSocketEvents';
import { queryKeys } from '../api/queryKeys';
import { applyAlertChange, applyBootstrap, applyDeviceStatus, applyMetricUpdate } from '../api/liveCache';

/**
 * Routes WebSocket events into the TanStack Query cache, so every page that
 * reads devices/metrics/alerts sees live data without its own subscription
 * or throttled refetch.
 */
export function useLiveQueryBridge() {
  const client = useQueryClient();

  useSocketEvents({
    onMetricUpdate: (metric) => applyMetricUpdate(client, metric),
    onDeviceStatus: (event) => applyDeviceStatus(client, event),
    onBootstrap: (payload) => applyBootstrap(client, payload),
    onAlertTriggered: () => applyAlertChange(client),
    onAlertUpdated: () => applyAlertChange(client),
    onAlertResolved: () => applyAlertChange(client),
    onCaptureStatus: () => { void client.invalidateQueries({ queryKey: queryKeys.capture.sessions() }); },
    onPortsScanned: ({ deviceId }) => { void client.invalidateQueries({ queryKey: queryKeys.devices.ports(deviceId) }); },
    onRemoteStatus: () => { void client.invalidateQueries({ queryKey: queryKeys.remote.all }); },
    onRemoteMetrics: () => { void client.invalidateQueries({ queryKey: queryKeys.remote.all }); },
  });
}
