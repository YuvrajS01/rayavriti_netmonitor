import { Suspense, useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import { deleteDevice, getDevice, getDeviceDependencies, getDeviceMetrics, getDevicePorts, getDevices, getLatestMetrics, scanDevicePorts, updateDevice } from '../api/devices';
import { getAlerts, ALERT_LIST_LIMIT } from '../api/alerts';
import { getHealthScore } from '../api/health';
import { getMetricsHistory, historyWindow, HISTORY_RANGES } from '../api/history';
import { queryKeys } from '../api/queryKeys';
import type { Alert, Device, DeviceDependencyNode, DeviceHistory, Metric, MetricMessagePayload, PortScanResult } from '../api/types';
import SectionHeader from '../components/ui/SectionHeader';
import Button from '../components/ui/Button';
import EmptyState from '../components/ui/EmptyState';
import ErrorState from '../components/ui/ErrorState';
import LoadingState from '../components/ui/LoadingState';
import RingGauge from '../components/ui/RingGauge';
import RelativeTime from '../components/ui/RelativeTime';
import TypeToConfirmDialog from '../components/ui/TypeToConfirmDialog';
import { StatusPill } from '../components/ui/Status';
import { Tabs, TabPanel } from '../components/ui/Tabs';
import { useToast } from '../components/ui/useToast';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { useAlertActions } from '../hooks/useAlertActions';
import { useResourceList } from '../hooks/useResources';
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { formatBytes } from '../utils/formatters';
import { lifecycleMeta, normalizeStatus, severityMeta } from '../utils/status';
import { hasPermission } from '../navigation';
import { useSessionUser } from '../store/session';

const ResponseHistoryChart = lazyWithRetry(() => import('../components/charts/ResponseHistoryChart'));

const TAB_IDS = ['overview', 'interfaces', 'alerts', 'ports', 'dependencies', 'settings'] as const;
type TabId = (typeof TAB_IDS)[number];

const RECENT_SAMPLES = 50;
const EMPTY_ALERTS: Alert[] = [];

const address = (d: Device) =>
  `${d.protocol === 'http' || d.protocol === 'https' ? `${d.protocol}://${d.ipAddress}` : d.ipAddress}${d.port > 0 && !['http', 'https'].includes(d.protocol) ? `:${d.port}` : ''}`;

function Tile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-surface-1 p-4">
      <p className="mb-1.5 text-xs text-fg-muted">{label}</p>
      <div className="font-semibold tabular-nums text-fg">{children}</div>
    </div>
  );
}

function interfacesOf(metric: Metric | undefined): MetricMessagePayload['interfaces'] {
  const details = metric?.details as MetricMessagePayload | undefined;
  return Array.isArray(details?.interfaces) ? details.interfaces : undefined;
}

export default function DeviceDetail() {
  const { id: rawId } = useParams();
  const id = Number(rawId);
  const user = useSessionUser();
  const queryClient = useQueryClient();
  const [tab, setTab] = useTabParam<TabId>(TAB_IDS);
  const tabsId = useTabsId();

  const listed = queryClient.getQueryData<{ data: Device[] }>(queryKeys.devices.list())?.data.find((d) => d.id === id);
  const deviceQuery = useQuery({
    queryKey: queryKeys.devices.detail(id),
    queryFn: ({ signal }) => getDevice(id, { signal }).then((r) => r.data),
    placeholderData: listed,
    enabled: Number.isFinite(id),
  });
  const latestQuery = useQuery({ queryKey: queryKeys.metrics.latest, queryFn: ({ signal }) => getLatestMetrics({ signal }) });
  const healthQuery = useQuery({
    queryKey: queryKeys.devices.health(id),
    queryFn: ({ signal }) => getHealthScore(id, { signal }),
    retry: false,
    enabled: Number.isFinite(id),
  });

  const device = deviceQuery.data;
  const latest = latestQuery.data?.data.find((m) => m.deviceId === id);
  const live = normalizeStatus(latest?.status || device?.status);

  if (!Number.isFinite(id) || (deviceQuery.isError && isAxiosError(deviceQuery.error) && deviceQuery.error.response?.status === 404)) {
    return (
      <EmptyState icon="search_off" title="Device not found" description="It may have been deleted, or it is outside your scope." action={<Link to="/devices" className="text-sm text-primary underline">Back to devices</Link>} />
    );
  }
  if (deviceQuery.isError && !device) return <ErrorState message="Failed to load the device." onRetry={() => void deviceQuery.refetch()} />;
  if (!device) return <LoadingState message="Loading device…" />;

  const healthScore = healthQuery.data?.data?.score ?? null;

  return (
    <div>
      <SectionHeader
        current={device.name}
        title={
          <span className="flex flex-wrap items-center gap-3">
            {device.name}
            <StatusPill status={live} size="md" />
          </span>
        }
        subtitle={<span className="font-mono">{address(device)} · <span className="uppercase">{device.protocol}</span></span>}
        action={
          <div className="flex items-center gap-3" title={healthScore == null ? 'No health score has been computed for this device yet' : undefined}>
            {healthScore != null ? (
              <RingGauge value={healthScore} size={56} strokeWidth={5} decimals={0} valueClassName="text-sm" label="Health" />
            ) : (
              <span className="flex h-14 w-14 items-center justify-center rounded-full border border-line text-sm text-fg-muted" aria-label="Health score: not computed yet">—</span>
            )}
          </div>
        }
      />

      <Tabs
        label="Device sections"
        idBase={tabsId}
        value={tab}
        onChange={setTab}
        className="mb-6"
        items={[
          { id: 'overview', label: 'Overview', icon: 'monitoring' },
          { id: 'interfaces', label: 'Interfaces', icon: 'settings_ethernet' },
          { id: 'alerts', label: 'Alerts', icon: 'notifications' },
          { id: 'ports', label: 'Ports', icon: 'lan' },
          { id: 'dependencies', label: 'Dependencies', icon: 'account_tree' },
          { id: 'settings', label: 'Settings', icon: 'tune' },
        ]}
      />

      <TabPanel idBase={tabsId} id="overview" value={tab}><OverviewTab device={device} latest={latest} /></TabPanel>
      <TabPanel idBase={tabsId} id="interfaces" value={tab}><InterfacesTab device={device} /></TabPanel>
      <TabPanel idBase={tabsId} id="alerts" value={tab}><AlertsTab device={device} /></TabPanel>
      <TabPanel idBase={tabsId} id="ports" value={tab}><PortsTab device={device} canScan={hasPermission(user, 'devices.write')} /></TabPanel>
      <TabPanel idBase={tabsId} id="dependencies" value={tab}><DependenciesTab device={device} /></TabPanel>
      <TabPanel idBase={tabsId} id="settings" value={tab}><SettingsTab device={device} /></TabPanel>
    </div>
  );
}

// ── Overview ──────────────────────────────────────────────────────────────

function OverviewTab({ device, latest }: { device: Device; latest?: Metric }) {
  const [rangeId, setRangeId] = useState('24h');
  const range = HISTORY_RANGES.find((r) => r.id === rangeId) ?? HISTORY_RANGES[2]!;
  const params = { devices: [device.id], ...historyWindow(range.ms), bucket: range.bucket };
  const history = useQuery({
    queryKey: queryKeys.metrics.history(params),
    queryFn: ({ signal }) => getMetricsHistory(params, { signal }),
    retry: false,
  });
  // v5.0 servers have no history endpoint: fall back to the latest raw polls.
  const recent = useQuery({
    queryKey: queryKeys.devices.metrics(device.id, RECENT_SAMPLES),
    queryFn: ({ signal }) => getDeviceMetrics(device.id, RECENT_SAMPLES, { signal }).then((r) => [...(r.data || [])].reverse()),
    enabled: history.isError,
  });
  const locations = useResourceList('/locations');

  const series = history.data ?? [];

  const fallbackSeries = useMemo<DeviceHistory[]>(() => {
    const rows = recent.data ?? [];
    if (!rows.length) return [];
    return [{
      deviceId: device.id,
      deviceName: device.name,
      points: rows.map((m) => ({
        t: m.timestamp || m.createdAt,
        avg: m.responseTime,
        min: m.responseTime,
        max: m.responseTime,
        samples: 1,
        down: normalizeStatus(m.status) === 'down' ? 1 : 0,
      })),
    }];
  }, [recent.data, device.id, device.name]);

  // Stats come from whichever series is on screen.
  const shown = history.isError ? fallbackSeries : series;
  const points = shown[0]?.points ?? [];
  const samples = points.reduce((n, p) => n + p.samples, 0);
  const down = points.reduce((n, p) => n + p.down, 0);
  const answered = points.filter((p) => p.avg != null);
  const answeredSamples = answered.reduce((n, p) => n + p.samples, 0);
  const availability = samples ? ((samples - down) / samples) * 100 : null;
  const avg = answeredSamples ? answered.reduce((n, p) => n + (p.avg ?? 0) * p.samples, 0) / answeredSamples : null;
  const windowLabel = history.isError ? `last ${RECENT_SAMPLES} polls` : range.label;

  const location = device.locationId != null ? (locations.data ?? []).find((l) => Number(l.id) === device.locationId) : undefined;
  const details: Array<[string, React.ReactNode]> = [
    ['Location', location ? String(location.name) : device.locationId != null ? `#${device.locationId}` : 'Unassigned'],
    ['Category', device.deviceCategory || '—'],
    ['Vendor', [device.manufacturer, device.model].filter(Boolean).join(' ') || '—'],
    ['Serial', device.serialNumber || '—'],
    ['Asset tag', device.assetTag || '—'],
    ['MAC address', device.macAddress ? <span className="font-mono">{device.macAddress}</span> : '—'],
    ['Rack position', device.rackPosition || '—'],
    ['Tags', device.tags?.length ? device.tags.join(', ') : '—'],
  ];

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        <Tile label="Last response">{latest?.responseTime != null ? `${latest.responseTime} ms` : '—'}</Tile>
        <Tile label="Last poll"><RelativeTime value={latest ? latest.timestamp || latest.createdAt : null} /></Tile>
        <Tile label={`Availability (${windowLabel})`}>{availability != null ? `${availability.toFixed(availability >= 99.95 ? 2 : 1)}%` : '—'}</Tile>
        <Tile label={`Average (${windowLabel})`}>{avg != null ? `${Math.round(avg)} ms` : '—'}</Tile>
        <Tile label="Failed polls">{samples ? `${down} of ${samples}` : '—'}</Tile>
        <Tile label="Poll interval">{device.interval}s</Tile>
      </div>

      <section aria-labelledby="latency-heading" className="rounded-lg border border-line bg-surface-1 p-5">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 id="latency-heading" className="text-sm font-semibold">Response time</h2>
            <p className="text-xs text-fg-muted">
              {history.isError ? `History is unavailable on this server; showing the last ${RECENT_SAMPLES} polls.` : 'Average per bucket with the min–max band; squares mark buckets with failed polls.'}
            </p>
          </div>
          <div role="radiogroup" aria-label="Range" className="flex gap-0.5 rounded-md border border-line bg-inset p-0.5">
            {HISTORY_RANGES.map((r) => (
              <button
                key={r.id}
                type="button"
                role="radio"
                aria-checked={r.id === rangeId}
                disabled={history.isError}
                onClick={() => setRangeId(r.id)}
                className={`h-8 rounded-[6px] px-3 text-sm font-medium transition-colors disabled:opacity-50 ${r.id === rangeId ? 'bg-surface-3 text-fg shadow-e1' : 'text-fg-muted hover:text-fg'}`}
              >
                {r.label}
              </button>
            ))}
          </div>
        </div>
        {history.isPending || (history.isError && recent.isPending) ? (
          <div className="flex h-64 items-center justify-center text-sm text-fg-muted" role="status">Loading response times…</div>
        ) : samples === 0 ? (
          <EmptyState size="compact" icon="monitoring" title="No samples in this range" description="The device hasn't been polled in this window." />
        ) : answered.length === 0 ? (
          <EmptyState size="compact" icon="dangerous" title="No responses in this range" description={`All ${samples} polls failed, so there is no response time to plot.`} />
        ) : (
          <Suspense fallback={<div className="h-64" />}>
            <ResponseHistoryChart series={shown} showRange={!history.isError} title={`Response time for ${device.name}`} />
          </Suspense>
        )}
      </section>

      <section aria-labelledby="details-heading" className="rounded-lg border border-line bg-surface-1 p-5">
        <h2 id="details-heading" className="mb-4 text-sm font-semibold">Details</h2>
        <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {details.map(([k, v]) => (
            <div key={k}>
              <dt className="text-xs text-fg-muted">{k}</dt>
              <dd className="mt-1 text-sm text-fg">{v}</dd>
            </div>
          ))}
        </dl>
        {device.notes && <p className="mt-4 whitespace-pre-line border-t border-line pt-4 text-sm text-fg-muted">{device.notes}</p>}
      </section>
    </div>
  );
}

// ── Interfaces ────────────────────────────────────────────────────────────

function InterfacesTab({ device }: { device: Device }) {
  const recent = useQuery({
    queryKey: queryKeys.devices.metrics(device.id, RECENT_SAMPLES),
    queryFn: ({ signal }) => getDeviceMetrics(device.id, RECENT_SAMPLES, { signal }).then((r) => [...(r.data || [])].reverse()),
  });
  if (recent.isPending) return <LoadingState message="Loading interfaces…" />;
  if (recent.isError) return <ErrorState message="Failed to load recent polls." onRetry={() => void recent.refetch()} />;

  const withIfaces = (recent.data ?? []).filter((m) => interfacesOf(m)?.length);
  const last = withIfaces[withIfaces.length - 1];
  const prev = withIfaces[withIfaces.length - 2];
  const ifaces = interfacesOf(last) ?? [];
  if (!ifaces.length) {
    return (
      <EmptyState
        icon="settings_ethernet"
        title="No interface data"
        description={device.protocol === 'snmp' ? 'No SNMP poll has returned interface counters yet.' : 'Interface counters are collected for SNMP devices only.'}
      />
    );
  }
  const seconds = last && prev ? (new Date(last.timestamp).getTime() - new Date(prev.timestamp).getTime()) / 1000 : 0;
  const prevByIndex = new Map((interfacesOf(prev) ?? []).map((i) => [i.index, i]));
  const rate = (now: number, before: number | undefined) =>
    seconds > 0 && before != null && now >= before ? `${(((now - before) * 8) / seconds / 1_000_000).toFixed(2)} Mbps` : '—';

  return (
    <div className="overflow-x-auto rounded-lg border border-line bg-surface-1">
      <table className="w-full text-left text-sm">
        <caption className="sr-only">Interfaces reported by the last SNMP poll</caption>
        <thead>
          <tr className="border-b border-line-strong text-xs text-fg-muted">
            <th scope="col" className="px-4 py-2.5 font-semibold">Interface</th>
            <th scope="col" className="px-4 py-2.5 font-semibold">ifIndex</th>
            <th scope="col" className="px-4 py-2.5 font-semibold">Status</th>
            <th scope="col" className="px-4 py-2.5 text-right font-semibold">Speed</th>
            <th scope="col" className="px-4 py-2.5 text-right font-semibold">In</th>
            <th scope="col" className="px-4 py-2.5 text-right font-semibold">Out</th>
          </tr>
        </thead>
        <tbody>
          {ifaces.map((i) => {
            const before = prevByIndex.get(i.index);
            return (
              <tr key={i.index} className="border-b border-line last:border-0">
                <td className="px-4 py-2.5 font-medium">{i.name}</td>
                <td className="px-4 py-2.5 font-mono text-fg-muted">{i.index}</td>
                <td className="px-4 py-2.5">{i.operStatus === 1 ? <StatusPill status="up" /> : i.operStatus === 2 ? <StatusPill status="down" /> : <StatusPill status="unknown" />}</td>
                <td className="px-4 py-2.5 text-right font-mono">{i.speed ? `${Math.round(i.speed / 1_000_000)} Mbps` : '—'}</td>
                <td className="px-4 py-2.5 text-right font-mono" title={formatBytes(i.inOctets)}>{rate(i.inOctets, before?.inOctets)}</td>
                <td className="px-4 py-2.5 text-right font-mono" title={formatBytes(i.outOctets)}>{rate(i.outOctets, before?.outOctets)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      <p className="border-t border-line px-4 py-2 text-xs text-fg-muted">Rates are computed from the last two polls{last ? <> (<RelativeTime value={last.timestamp} />)</> : null}.</p>
    </div>
  );
}

// ── Alerts ────────────────────────────────────────────────────────────────

function AlertsTab({ device }: { device: Device }) {
  const user = useSessionUser();
  const { act } = useAlertActions();
  // Same keys as the Alerts page, so these stay live via the cache bridge.
  // GET /alerts has no device filter; filter the (≤200) open alerts here.
  const active = useQuery({ queryKey: queryKeys.alerts.list('active', ALERT_LIST_LIMIT), queryFn: ({ signal }) => getAlerts('active', ALERT_LIST_LIMIT, { signal }) });
  const acked = useQuery({ queryKey: queryKeys.alerts.list('acknowledged', ALERT_LIST_LIMIT), queryFn: ({ signal }) => getAlerts('acknowledged', ALERT_LIST_LIMIT, { signal }) });
  const resolved = useQuery({ queryKey: queryKeys.alerts.list('resolved', ALERT_LIST_LIMIT), queryFn: ({ signal }) => getAlerts('resolved', ALERT_LIST_LIMIT, { signal }) });
  const alerts = useMemo(
    () =>
      [...(active.data?.data ?? EMPTY_ALERTS), ...(acked.data?.data ?? EMPTY_ALERTS), ...(resolved.data?.data ?? EMPTY_ALERTS)]
        .filter((a) => a.deviceId === device.id)
        .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()),
    [active.data, acked.data, resolved.data, device.id],
  );
  const canAck = hasPermission(user, 'alerts.acknowledge');
  const canResolve = hasPermission(user, 'alerts.resolve');

  if (active.isPending) return <LoadingState message="Loading alerts…" />;
  if (active.isError) return <ErrorState message="Failed to load alerts." onRetry={() => void active.refetch()} />;
  if (!alerts.length) return <EmptyState icon="notifications_off" title="No recent alerts" description={`Nothing has fired for ${device.name} recently.`} />;

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <Link to={`/alerts?q=${encodeURIComponent(device.name)}`} className="text-sm text-primary underline">Open in the alerts workbench</Link>
      </div>
      <ul className="divide-y divide-line rounded-lg border border-line bg-surface-1">
        {alerts.map((a) => {
          const meta = severityMeta(a.severity);
          return (
            <li key={a.id} className={`flex flex-col gap-3 border-l-4 px-4 py-3 md:flex-row md:items-center ${meta.borderL}`}>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <StatusPill status={meta} />
                  <StatusPill status={lifecycleMeta(a.status)} />
                  <span className="font-medium text-fg">{a.message}</span>
                </div>
                <p className="mt-1 text-xs text-fg-muted">
                  <RelativeTime value={a.createdAt} />
                  {a.acknowledgedBy && ` · acknowledged by ${a.acknowledgedBy}`}
                  {a.assignedToName && ` · assigned to ${a.assignedToName}`}
                </p>
              </div>
              <div className="flex gap-2">
                {a.status === 'active' && canAck && <Button size="sm" variant="secondary" icon="check" onClick={() => act(a.id, 'acknowledge')} aria-label={`Acknowledge alert: ${a.message}`}>Acknowledge</Button>}
                {a.status !== 'resolved' && canResolve && <Button size="sm" variant="primary-outline" icon="done_all" onClick={() => act(a.id, 'resolve')} aria-label={`Resolve alert: ${a.message}`}>Resolve</Button>}
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

// ── Ports ─────────────────────────────────────────────────────────────────

function PortsTab({ device, canScan }: { device: Device; canScan: boolean }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const key = queryKeys.devices.ports(device.id);
  const ports = useQuery({ queryKey: key, queryFn: ({ signal }) => getDevicePorts(device.id, { signal }).then((r) => r.data || []) });
  const scan = useMutation({
    mutationFn: () => scanDevicePorts(device.id),
    onSuccess: (res) => {
      queryClient.setQueryData<PortScanResult[]>(key, res.data.results.map((r) => ({ ...r, deviceId: device.id })));
      addToast(`Scan finished: ${res.data.openPorts} open of ${res.data.scannedPorts}`, 'success');
    },
    onError: () => addToast('Port scan failed', 'error'),
  });
  const list = ports.data ?? [];
  const open = list.filter((p) => p.state === 'open');

  return (
    <section aria-labelledby="ports-heading" className="rounded-lg border border-line bg-surface-1">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line p-4">
        <div>
          <h2 id="ports-heading" className="text-sm font-semibold">Port inventory</h2>
          <p className="text-xs text-fg-muted">{open.length} open of {list.length} scanned ports</p>
        </div>
        {canScan && (
          <Button size="sm" variant="secondary" icon={scan.isPending ? 'hourglass_top' : 'radar'} onClick={() => scan.mutate()} disabled={scan.isPending}>
            {scan.isPending ? 'Scanning…' : 'Scan ports'}
          </Button>
        )}
      </div>
      {ports.isPending ? (
        <LoadingState message="Loading ports…" />
      ) : ports.isError ? (
        <div className="p-4"><ErrorState message="Failed to load port results." onRetry={() => void ports.refetch()} /></div>
      ) : list.length === 0 ? (
        <EmptyState size="compact" icon="lan" title="No port scan results yet" description={canScan ? 'Run a scan to inventory listening services.' : undefined} />
      ) : (
        <table className="w-full text-left text-sm">
          <caption className="sr-only">Scanned ports</caption>
          <thead>
            <tr className="border-b border-line-strong text-xs text-fg-muted">
              <th scope="col" className="px-4 py-2.5 font-semibold">Port</th>
              <th scope="col" className="px-4 py-2.5 font-semibold">Service</th>
              <th scope="col" className="px-4 py-2.5 font-semibold">State</th>
              <th scope="col" className="px-4 py-2.5 text-right font-semibold">Response</th>
              <th scope="col" className="px-4 py-2.5 font-semibold">Last changed</th>
            </tr>
          </thead>
          <tbody>
            {[...list].sort((a, b) => a.port - b.port).map((p) => (
              <tr key={`${p.port}-${p.protocol}`} className="border-b border-line last:border-0">
                <td className="px-4 py-2.5 font-mono">{p.port}/{p.protocol}</td>
                <td className="px-4 py-2.5">{p.service || 'Unknown'}</td>
                <td className="px-4 py-2.5">{p.state === 'open' ? <StatusPill status="info">Open</StatusPill> : <StatusPill status="unknown">Closed</StatusPill>}</td>
                <td className="px-4 py-2.5 text-right font-mono">{typeof p.responseTime === 'number' ? `${p.responseTime} ms` : '—'}</td>
                <td className="px-4 py-2.5 text-fg-muted"><RelativeTime value={p.lastChangedAt || p.scannedAt || null} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

// ── Dependencies ──────────────────────────────────────────────────────────

function NodeLink({ node }: { node: DeviceDependencyNode }) {
  return (
    <span className="inline-flex items-center gap-2">
      <StatusPill status={node.status} />
      <Link to={`/devices/${node.deviceId}`} className="font-medium text-fg hover:underline">{node.name}</Link>
      <code className="font-mono text-xs text-fg-muted">{node.host}</code>
      {node.dependencyPort && <span className="text-xs text-fg-muted">via {node.dependencyPort}</span>}
    </span>
  );
}

function DependenciesTab({ device }: { device: Device }) {
  const deps = useQuery({
    queryKey: queryKeys.devices.dependencies(device.id),
    queryFn: ({ signal }) => getDeviceDependencies(device.id, { signal }).then((r) => r.data),
  });
  if (deps.isPending) return <LoadingState message="Loading dependencies…" />;
  if (deps.isError) return <ErrorState message="Failed to load dependencies." onRetry={() => void deps.refetch()} />;
  // Ancestors arrive nearest-first; show the path from the root down.
  const ancestors = [...(deps.data?.ancestors ?? [])].reverse();
  const descendants = deps.data?.descendants ?? [];
  const children = descendants.filter((d) => d.parentDeviceId === device.id);
  const deeper = descendants.length - children.length;

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <section aria-labelledby="upstream-heading" className="rounded-lg border border-line bg-surface-1 p-5">
        <h2 id="upstream-heading" className="mb-1 text-sm font-semibold">Upstream path</h2>
        <p className="mb-4 text-xs text-fg-muted">If any of these goes down, alerts for {device.name} are suppressed as a downstream effect.</p>
        {ancestors.length === 0 ? (
          <p className="text-sm text-fg-muted">This device is a root: it has no parent.</p>
        ) : (
          <ol className="space-y-2">
            {ancestors.map((a, i) => (
              <li key={a.deviceId} className="flex items-center gap-2" style={{ paddingLeft: `${i * 16}px` }}>
                <span className="material-symbols-outlined text-[16px] text-fg-subtle" aria-hidden="true">subdirectory_arrow_right</span>
                <NodeLink node={a} />
              </li>
            ))}
            <li className="flex items-center gap-2 font-semibold" style={{ paddingLeft: `${ancestors.length * 16}px` }}>
              <span className="material-symbols-outlined text-[16px] text-fg-subtle" aria-hidden="true">subdirectory_arrow_right</span>
              {device.name} <span className="text-xs font-normal text-fg-muted">(this device)</span>
            </li>
          </ol>
        )}
      </section>
      <section aria-labelledby="downstream-heading" className="rounded-lg border border-line bg-surface-1 p-5">
        <h2 id="downstream-heading" className="mb-1 text-sm font-semibold">Downstream devices</h2>
        <p className="mb-4 text-xs text-fg-muted">{children.length} direct {children.length === 1 ? 'child' : 'children'}{deeper > 0 ? `, ${deeper} further down` : ''}.</p>
        {children.length === 0 ? (
          <p className="text-sm text-fg-muted">Nothing depends on this device.</p>
        ) : (
          <ul className="space-y-2">{children.map((c) => <li key={c.deviceId}><NodeLink node={c} /></li>)}</ul>
        )}
      </section>
    </div>
  );
}

// ── Settings ──────────────────────────────────────────────────────────────

function Field({ id, label, children, hint }: { id: string; label: string; children: React.ReactNode; hint?: React.ReactNode }) {
  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-xs text-fg-muted">{label}</label>
      {children}
      {hint}
    </div>
  );
}

const PROTOCOLS = ['ping', 'icmp', 'http', 'https', 'port', 'snmp', 'ssh', 'camera', 'biometric'];

function SettingsTab({ device }: { device: Device }) {
  const user = useSessionUser();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const canWrite = hasPermission(user, 'devices.write');
  const canDelete = hasPermission(user, 'devices.delete');
  const locations = useResourceList('/locations');
  const devices = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }) });
  const [confirmDelete, setConfirmDelete] = useState(false);
  const initial = {
    name: device.name,
    ipAddress: device.ipAddress,
    port: device.port ? String(device.port) : '',
    protocol: device.protocol,
    interval: String(device.interval),
    locationId: device.locationId != null ? String(device.locationId) : '',
    parentDeviceId: device.parentDeviceId != null ? String(device.parentDeviceId) : '',
    dependencyPort: device.dependencyPort || '',
    manufacturer: device.manufacturer || '',
    model: device.model || '',
    serialNumber: device.serialNumber || '',
    macAddress: device.macAddress || '',
    tags: (device.tags || []).join(', '),
    notes: device.notes || '',
  };
  const [form, setForm] = useState(initial);
  const set = (k: keyof typeof initial) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));
  const intervalInvalid = !/^\d+$/.test(form.interval) || Number(form.interval) < 5;
  const portInvalid = form.port !== '' && (!/^\d+$/.test(form.port) || Number(form.port) > 65535);

  const save = useMutation({
    mutationFn: () => {
      // Only send what changed: the API ignores empty strings, and placement
      // fields are only applied when present.
      const patch: Record<string, unknown> = {};
      (Object.keys(initial) as Array<keyof typeof initial>).forEach((k) => {
        if (form[k] === initial[k]) return;
        if (k === 'port' || k === 'interval') patch[k] = Number(form[k]);
        else if (k === 'locationId' || k === 'parentDeviceId') patch[k] = form[k] ? Number(form[k]) : null;
        else if (k === 'tags') patch.tags = form.tags.split(',').map((t) => t.trim()).filter(Boolean);
        else patch[k] = form[k];
      });
      return updateDevice(device.id, patch);
    },
    onSuccess: () => {
      addToast('Device saved', 'success');
      void queryClient.invalidateQueries({ queryKey: queryKeys.devices.all });
    },
    onError: (error) => {
      const message = isAxiosError(error) ? (error.response?.data as { error?: { message?: string } } | undefined)?.error?.message : undefined;
      addToast(message ? `Save failed: ${message}` : 'Save failed', 'error');
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteDevice(device.id),
    onSuccess: () => {
      addToast(`Deleted "${device.name}"`, 'success');
      void queryClient.invalidateQueries({ queryKey: queryKeys.devices.all });
      void navigate('/devices');
    },
    onError: () => addToast('Delete failed', 'error'),
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (intervalInvalid || portInvalid) return;
    save.mutate();
  };

  const field = 'h-control w-full rounded-md border border-line bg-inset px-3 text-sm text-fg outline-none focus:border-primary disabled:opacity-60 aria-[invalid=true]:border-error';
  const dirty = JSON.stringify(form) !== JSON.stringify(initial);

  return (
    <div className="space-y-6">
      <form onSubmit={submit} className="rounded-lg border border-line bg-surface-1 p-5" aria-labelledby="settings-heading">
        <h2 id="settings-heading" className="mb-4 text-sm font-semibold">Monitoring</h2>
        <fieldset disabled={!canWrite} className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          <Field id="dev-name" label="Name"><input id="dev-name" value={form.name} onChange={set('name')} required maxLength={255} className={field} /></Field>
          <Field id="dev-ip" label="Address (IP or host)"><input id="dev-ip" value={form.ipAddress} onChange={set('ipAddress')} required className={`${field} font-mono`} /></Field>
          <Field id="dev-proto" label="Protocol">
            <select id="dev-proto" value={form.protocol} onChange={set('protocol')} className={field}>
              {PROTOCOLS.map((p) => <option key={p} value={p}>{p.toUpperCase()}</option>)}
            </select>
          </Field>
          <Field id="dev-port" label="Port" hint={portInvalid && <p className="mt-1 text-xs text-error">Port must be 1–65535.</p>}>
            <input id="dev-port" inputMode="numeric" value={form.port} onChange={set('port')} aria-invalid={portInvalid} className={`${field} font-mono`} />
          </Field>
          <Field id="dev-interval" label="Poll interval (seconds)" hint={intervalInvalid && <p className="mt-1 text-xs text-error">At least 5 seconds.</p>}>
            <input id="dev-interval" inputMode="numeric" value={form.interval} onChange={set('interval')} aria-invalid={intervalInvalid} className={`${field} font-mono`} />
          </Field>
          <Field id="dev-loc" label="Location">
            <select id="dev-loc" value={form.locationId} onChange={set('locationId')} className={field}>
              <option value="">Unassigned</option>
              {(locations.data ?? []).map((l) => <option key={String(l.id)} value={String(l.id)}>{String(l.name)}</option>)}
            </select>
          </Field>
          <Field id="dev-parent" label="Parent device (dependency)">
            <select id="dev-parent" value={form.parentDeviceId} onChange={set('parentDeviceId')} className={field}>
              <option value="">None (root)</option>
              {(devices.data?.data ?? []).filter((d) => d.id !== device.id).map((d) => <option key={d.id} value={String(d.id)}>{d.name} ({d.ipAddress})</option>)}
            </select>
          </Field>
          <Field id="dev-depport" label="Dependency port"><input id="dev-depport" value={form.dependencyPort} onChange={set('dependencyPort')} placeholder="e.g. eth0, 1/0/1" className={`${field} font-mono`} /></Field>
          <Field id="dev-tags" label="Tags (comma separated)"><input id="dev-tags" value={form.tags} onChange={set('tags')} className={field} /></Field>
        </fieldset>

        <h2 className="mb-4 mt-6 text-sm font-semibold">Asset</h2>
        <fieldset disabled={!canWrite} className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Field id="dev-vendor" label="Vendor"><input id="dev-vendor" value={form.manufacturer} onChange={set('manufacturer')} className={field} /></Field>
          <Field id="dev-model" label="Model"><input id="dev-model" value={form.model} onChange={set('model')} className={field} /></Field>
          <Field id="dev-serial" label="Serial number"><input id="dev-serial" value={form.serialNumber} onChange={set('serialNumber')} className={`${field} font-mono`} /></Field>
          <Field id="dev-mac" label="MAC address"><input id="dev-mac" value={form.macAddress} onChange={set('macAddress')} className={`${field} font-mono`} /></Field>
          <div className="md:col-span-2 xl:col-span-4">
            <label htmlFor="dev-notes" className="mb-1.5 block text-xs text-fg-muted">Notes</label>
            <textarea id="dev-notes" value={form.notes} onChange={set('notes')} rows={3} className="w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary disabled:opacity-60" />
          </div>
        </fieldset>
        <p className="mt-3 text-xs text-fg-muted">Clearing a text field does not erase the stored value (the API ignores empty strings).</p>
        {canWrite && (
          <div className="mt-5 flex gap-2">
            <Button type="submit" icon="save" disabled={!dirty || save.isPending || intervalInvalid || portInvalid}>{save.isPending ? 'Saving…' : 'Save changes'}</Button>
            <Button type="button" variant="ghost" disabled={!dirty} onClick={() => setForm(initial)}>Discard</Button>
          </div>
        )}
      </form>

      {canDelete && (
        <section aria-labelledby="danger-heading" className="rounded-lg border border-line border-l-4 border-l-error bg-surface-1 p-5">
          <h2 id="danger-heading" className="mb-1 text-sm font-semibold">Delete device</h2>
          <p className="mb-4 text-sm text-fg-muted">Removes the device with its metrics history, port scans and alert history.</p>
          <Button variant="danger-outline" icon="delete" onClick={() => setConfirmDelete(true)}>Delete {device.name}…</Button>
        </section>
      )}

      <TypeToConfirmDialog
        open={confirmDelete}
        title={`Delete ${device.name}?`}
        message="This cannot be undone."
        phrase={device.name}
        confirmLabel="Delete device"
        busy={remove.isPending}
        onConfirm={() => remove.mutate()}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  );
}
