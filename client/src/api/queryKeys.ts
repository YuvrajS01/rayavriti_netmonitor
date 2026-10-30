import type { LogFilters } from './logs';
import type { HistoryParams } from './history';

/**
 * Every TanStack Query key in the app. Keys are hierarchical so that
 * invalidating a prefix (e.g. `queryKeys.alerts.all`) refreshes every
 * dependent query.
 */
export const queryKeys = {
  auth: {
    session: ['auth', 'session'] as const,
  },
  devices: {
    all: ['devices'] as const,
    list: () => [...queryKeys.devices.all, 'list'] as const,
    metrics: (id: number, limit?: number) => [...queryKeys.devices.all, id, 'metrics', limit ?? null] as const,
    ports: (id: number) => [...queryKeys.devices.all, id, 'ports'] as const,
    health: (id: number) => [...queryKeys.devices.all, id, 'health'] as const,
    detail: (id: number) => [...queryKeys.devices.all, id, 'detail'] as const,
    dependencies: (id: number) => [...queryKeys.devices.all, id, 'dependencies'] as const,
    interfaces: (id: number) => [...queryKeys.devices.all, id, 'interfaces'] as const,
    neighbors: (id: number) => [...queryKeys.devices.all, id, 'neighbors'] as const,
  },
  metrics: {
    latest: ['metrics', 'latest'] as const,
    history: (params: HistoryParams) => ['metrics', 'history', params] as const,
  },
  search: {
    query: (q: string) => ['search', q] as const,
  },
  topology: {
    tree: ['topology', 'tree'] as const,
  },
  alerts: {
    all: ['alerts'] as const,
    list: (status: string, limit?: number) => [...queryKeys.alerts.all, 'list', status, limit ?? null] as const,
    grouped: (status: string) => [...queryKeys.alerts.all, 'grouped', status] as const,
    counts: () => [...queryKeys.alerts.all, 'counts'] as const,
    notes: (id: number) => [...queryKeys.alerts.all, 'notes', id] as const,
    assignable: () => [...queryKeys.alerts.all, 'assignable-users'] as const,
  },
  system: {
    stats: ['system', 'stats'] as const,
    info: ['system', 'info'] as const,
  },
  insights: {
    all: ['insights'] as const,
    current: () => [...queryKeys.insights.all, 'current'] as const,
    history: (hours: number) => [...queryKeys.insights.all, 'history', hours] as const,
  },
  flows: {
    all: ['flows'] as const,
    stats: () => [...queryKeys.flows.all, 'stats'] as const,
    timeseries: (bucketMinutes: number) => [...queryKeys.flows.all, 'timeseries', bucketMinutes] as const,
    topTalkers: (direction: 'src' | 'dst', limit: number) => [...queryKeys.flows.all, 'top-talkers', direction, limit] as const,
    protocols: () => [...queryKeys.flows.all, 'protocols'] as const,
    records: (limit: number) => [...queryKeys.flows.all, 'records', limit] as const,
  },
  capture: {
    all: ['capture'] as const,
    interfaces: () => [...queryKeys.capture.all, 'interfaces'] as const,
    sessions: () => [...queryKeys.capture.all, 'sessions'] as const,
  },
  reports: {
    all: ['reports'] as const,
    summary: (query: string) => [...queryKeys.reports.all, 'summary', query] as const,
    timeseries: (query: string) => [...queryKeys.reports.all, 'timeseries', query] as const,
    devices: (query: string) => [...queryKeys.reports.all, 'devices', query] as const,
    alerts: (query: string) => [...queryKeys.reports.all, 'alerts', query] as const,
    isp: (query: string) => [...queryKeys.reports.all, 'isp', query] as const,
    scheduled: () => [...queryKeys.reports.all, 'scheduled'] as const,
    generated: () => [...queryKeys.reports.all, 'generated'] as const,
  },
  logs: {
    all: ['logs'] as const,
    events: (filters: LogFilters) => [...queryKeys.logs.all, 'events', filters] as const,
    stats: (filters: LogFilters) => [...queryKeys.logs.all, 'stats', filters] as const,
    verboseSessions: () => [...queryKeys.logs.all, 'verbose-sessions'] as const,
  },
  incidents: {
    all: ['incidents'] as const,
    list: () => [...queryKeys.incidents.all, 'list'] as const,
    stats: () => [...queryKeys.incidents.all, 'stats'] as const,
    detail: (id: string) => [...queryKeys.incidents.all, 'detail', id] as const,
    timeline: (id: string) => [...queryKeys.incidents.all, 'detail', id, 'timeline'] as const,
    devices: (id: string) => [...queryKeys.incidents.all, 'detail', id, 'devices'] as const,
  },
  isp: {
    all: ['isp'] as const,
    links: () => [...queryKeys.isp.all, 'links'] as const,
    comparison: () => [...queryKeys.isp.all, 'comparison'] as const,
    sla: (id: number) => [...queryKeys.isp.all, id, 'sla'] as const,
    timeseries: (id: number) => [...queryKeys.isp.all, id, 'timeseries'] as const,
  },
  discovery: {
    all: ['discovery'] as const,
    jobs: () => [...queryKeys.discovery.all, 'jobs'] as const,
    results: (jobId: number) => [...queryKeys.discovery.all, 'results', jobId] as const,
  },
  users: {
    all: ['users'] as const,
    list: () => [...queryKeys.users.all, 'list'] as const,
    roles: () => [...queryKeys.users.all, 'roles'] as const,
  },
  statusPage: {
    all: ['status-page'] as const,
    services: () => [...queryKeys.statusPage.all, 'services'] as const,
    incidents: () => [...queryKeys.statusPage.all, 'incidents'] as const,
  },
  backups: {
    all: ['backups'] as const,
    list: () => [...queryKeys.backups.all, 'list'] as const,
    config: () => [...queryKeys.backups.all, 'config'] as const,
  },
  serviceTemplates: {
    list: ['service-templates'] as const,
  },
  remote: {
    all: ['remote'] as const,
    overview: () => [...queryKeys.remote.all, 'overview'] as const,
    instances: () => [...queryKeys.remote.all, 'instances'] as const,
  },
  resources: {
    all: ['resources'] as const,
    /** Generic CRUD collections served by api/resources.ts, keyed by path. */
    list: (path: string) => [...queryKeys.resources.all, path] as const,
  },
} as const;
