import { Suspense, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getDevices } from '../../../api/devices';
import { getMetricsHistory, historyWindow, HISTORY_MAX_DEVICES, HISTORY_RANGES } from '../../../api/history';
import { getInterfaceHistory } from '../../../api/interfaces';
import { queryKeys } from '../../../api/queryKeys';
import type { DashboardWidget } from '../../../api/types';
import { useResourceList } from '../../../hooks/useResources';
import { lazyWithRetry } from '../../../utils/lazyWithRetry';
import { normalizeStatus } from '../../../utils/status';
import { useNow } from '../../../utils/time';
import ErrorState from '../../ui/ErrorState';
import { StatusDot } from '../../ui/Status';
import { useLatestMetrics } from '../useDashboardData';
import { widgetTitle } from './catalog';
import WidgetCard, { NeedsSetup } from './WidgetCard';
import { chartHeight, filterDevices } from './widgetData';

const ResponseHistoryChart = lazyWithRetry(() => import('../../charts/ResponseHistoryChart'));
const ThroughputChart = lazyWithRetry(() => import('./ThroughputChart'));

export interface WidgetProps {
  widget: DashboardWidget;
}

const num = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : undefined);
const rangeOf = (v: unknown) => HISTORY_RANGES.find((r) => r.id === v) ?? HISTORY_RANGES[2]!;

const chartFallback = (h: number) => <div role="status" className="grid place-items-center text-xs text-fg-muted" style={{ height: chartHeight(h) }}>Loading chart…</div>;

// ── Device response history ────────────────────────────────────────────────

export function DeviceHistoryWidget({ widget }: WidgetProps) {
  const config = widget.config ?? {};
  const deviceIds = useMemo(
    () => (Array.isArray(config.deviceIds) ? config.deviceIds.filter((v): v is number => typeof v === 'number') : []).slice(0, HISTORY_MAX_DEVICES),
    [config.deviceIds],
  );
  const range = rangeOf(config.range);
  const win = historyWindow(range.ms, useNow());
  const params = { devices: deviceIds, ...win, bucket: range.bucket };
  const history = useQuery({
    queryKey: queryKeys.metrics.history(params),
    queryFn: ({ signal }) => getMetricsHistory(params, { signal }),
    enabled: deviceIds.length > 0,
    placeholderData: keepPreviousData,
  });
  const title = widgetTitle(widget.type, config);

  return (
    <WidgetCard title={title} subtitle={deviceIds.length ? `Last ${range.label}, ${range.bucket} buckets` : undefined}>
      {!deviceIds.length ? (
        <NeedsSetup>Choose devices in this widget’s settings.</NeedsSetup>
      ) : history.isError ? (
        <ErrorState message="Failed to load response history." onRetry={() => void history.refetch()} />
      ) : history.isPending ? (
        chartFallback(widget.h)
      ) : !history.data?.some((s) => s.points.some((p) => p.samples > 0)) ? (
        <p className="py-10 text-center text-xs text-fg-muted">No polls in this range.</p>
      ) : (
        <Suspense fallback={chartFallback(widget.h)}>
          <ResponseHistoryChart series={history.data} height={chartHeight(widget.h)} showRange={history.data.length === 1} title={title} />
        </Suspense>
      )}
    </WidgetCard>
  );
}

// ── Interface throughput ───────────────────────────────────────────────────

export function InterfaceThroughputWidget({ widget }: WidgetProps) {
  const config = widget.config ?? {};
  const deviceId = num(config.deviceId);
  const ifIndex = num(config.ifIndex);
  const range = rangeOf(config.range);
  const win = historyWindow(range.ms, useNow());
  const configured = deviceId != null && ifIndex != null;
  const history = useQuery({
    queryKey: queryKeys.devices.interfaceHistory(deviceId ?? 0, ifIndex ?? 0, win.from, win.to),
    queryFn: ({ signal }) => getInterfaceHistory(deviceId!, ifIndex!, win.from, win.to, { signal }),
    enabled: configured,
    placeholderData: keepPreviousData,
  });
  const ifName = typeof config.ifName === 'string' ? config.ifName : ifIndex != null ? `ifIndex ${ifIndex}` : '';
  const deviceName = typeof config.deviceName === 'string' ? config.deviceName : '';

  return (
    <WidgetCard
      title={widgetTitle(widget.type, config)}
      subtitle={configured ? <>{deviceId != null && <Link to={`/devices/${deviceId}?tab=interfaces`} className="hover:underline">{deviceName || `Device ${deviceId}`}</Link>} · {ifName} · last {range.label}</> : undefined}
    >
      {!configured ? (
        <NeedsSetup>Choose an SNMP device and interface in this widget’s settings.</NeedsSetup>
      ) : history.isError ? (
        <ErrorState message="Failed to load interface throughput." onRetry={() => void history.refetch()} />
      ) : history.isPending ? (
        chartFallback(widget.h)
      ) : !history.data?.length ? (
        <p className="py-10 text-center text-xs text-fg-muted">No samples for this interface in this range.</p>
      ) : (
        <Suspense fallback={chartFallback(widget.h)}>
          <ThroughputChart points={history.data} height={chartHeight(widget.h)} title={`${deviceName} ${ifName} throughput`} />
        </Suspense>
      )}
    </WidgetCard>
  );
}

// ── Device list ────────────────────────────────────────────────────────────


export function DeviceListWidget({ widget }: WidgetProps) {
  const config = widget.config ?? {};
  const devicesQuery = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }) });
  const { metrics } = useLatestMetrics();
  const locationId = num(config.locationId);
  const locations = useResourceList('/locations', { enabled: locationId != null });
  const status = (['all', 'problems', 'down'] as const).find((s) => s === config.status) ?? 'all';
  const tag = typeof config.tag === 'string' ? config.tag : undefined;
  const limit = Math.min(50, Math.max(1, num(config.limit) ?? 10));

  const statusById = useMemo(() => new Map(metrics.map((m) => [m.deviceId, m.status])), [metrics]);
  const matching = useMemo(
    () => filterDevices(devicesQuery.data?.data ?? [], statusById, { status, locationId, tag }),
    [devicesQuery.data, statusById, status, locationId, tag],
  );
  const locationRow = locations.data?.find((l) => Number(l.id) === locationId);
  const locationName = locationId == null ? '' : typeof locationRow?.name === 'string' ? locationRow.name : `Location ${locationId}`;
  const filters = [locationName, tag && `Tag “${tag}”`, status === 'down' ? 'Down only' : status === 'problems' ? 'Not up' : ''].filter(Boolean).join(' · ');

  return (
    <WidgetCard title={widgetTitle(widget.type, config)} subtitle={filters || 'All devices'} action={<span className="rounded-full bg-surface-3 px-2 py-0.5 text-xs font-semibold tabular-nums text-fg">{matching.length}</span>}>
      {devicesQuery.isError ? (
        <ErrorState message="Failed to load devices." onRetry={() => void devicesQuery.refetch()} />
      ) : devicesQuery.isPending ? (
        <p role="status" className="py-6 text-center text-xs text-fg-muted">Loading devices…</p>
      ) : !matching.length ? (
        <p className="py-6 text-center text-sm text-fg-muted">{status === 'all' ? 'No devices match.' : 'Nothing matching is having problems.'}</p>
      ) : (
        <ul className="-mx-2 divide-y divide-line overflow-y-auto">
          {matching.slice(0, limit).map((d) => {
            const s = normalizeStatus(statusById.get(d.id) ?? d.status);
            return (
              <li key={d.id}>
                <Link to={`/devices/${d.id}`} className="flex items-center gap-2.5 rounded-sm px-2 py-2 hover:bg-surface-2">
                  <StatusDot status={s} />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium text-fg">{d.name}</span>
                  <span className="truncate font-mono text-xs text-fg-muted">{d.ipAddress}</span>
                </Link>
              </li>
            );
          })}
          {matching.length > limit && <li className="px-2 py-2 text-xs text-fg-muted">and {matching.length - limit} more</li>}
        </ul>
      )}
    </WidgetCard>
  );
}
