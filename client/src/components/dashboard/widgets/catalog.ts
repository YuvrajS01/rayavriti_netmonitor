/**
 * The widget catalog: what each widget type is, how big it starts and
 * whether it has settings. Components live in ./registry and load lazily,
 * so this list costs nothing on pages that don't edit dashboards.
 */
export type WidgetCategory = 'Summary' | 'Charts' | 'Lists' | 'Devices and interfaces';

export interface WidgetMeta {
  type: string;
  label: string;
  description: string;
  icon: string;
  category: WidgetCategory;
  defaultSize: { w: number; h: number };
  /** Widgets with fixed element ids can appear once per dashboard. */
  single?: boolean;
  /** Has a settings form (and needs settings before it shows data). */
  configurable?: boolean;
  defaultConfig?: Record<string, unknown>;
}

export const WIDGETS: readonly WidgetMeta[] = [
  { type: 'stat.total-devices', label: 'Total devices', description: 'How many devices are monitored.', icon: 'devices', category: 'Summary', defaultSize: { w: 3, h: 1 } },
  { type: 'stat.online', label: 'Online devices', description: 'Devices currently up, out of the total.', icon: 'check_circle', category: 'Summary', defaultSize: { w: 3, h: 1 } },
  { type: 'stat.uptime', label: 'Devices up', description: 'Share of devices up in the latest poll.', icon: 'percent', category: 'Summary', defaultSize: { w: 3, h: 1 } },
  { type: 'stat.active-alerts', label: 'Active alerts', description: 'Open alerts that need attention.', icon: 'notifications', category: 'Summary', defaultSize: { w: 3, h: 1 } },
  { type: 'health.score', label: 'Network health score', description: 'Average AI health score across devices.', icon: 'monitor_heart', category: 'Summary', defaultSize: { w: 4, h: 2 }, single: true },
  { type: 'insights', label: 'Smart insights', description: 'Anomalies and grouped risks detected right now.', icon: 'lightbulb', category: 'Summary', defaultSize: { w: 8, h: 2 }, single: true },

  { type: 'response.slowest', label: 'Response time, slowest devices', description: 'Last hour of response time for the six slowest devices.', icon: 'show_chart', category: 'Charts', defaultSize: { w: 8, h: 2 }, single: true },
  { type: 'status.distribution', label: 'Status distribution', description: 'Devices by status in the latest poll.', icon: 'donut_large', category: 'Charts', defaultSize: { w: 4, h: 2 }, single: true },
  { type: 'status.heatmap', label: 'Status pulse', description: 'Recent poll results per device.', icon: 'grid_on', category: 'Charts', defaultSize: { w: 4, h: 2 }, single: true },
  { type: 'resource.load', label: 'Server load', description: 'CPU and memory of the NetMonitor host, and the share of devices down.', icon: 'memory', category: 'Charts', defaultSize: { w: 6, h: 2 }, single: true },
  { type: 'response.by-status', label: 'Average response by status', description: 'Mean response time for each device status.', icon: 'bar_chart', category: 'Charts', defaultSize: { w: 6, h: 2 }, single: true },
  { type: 'topology.mini', label: 'Topology preview', description: 'A small view of the network map.', icon: 'hub', category: 'Charts', defaultSize: { w: 8, h: 2 }, single: true },

  { type: 'metrics.latest', label: 'Latest metrics', description: 'The newest poll of each device.', icon: 'table_rows', category: 'Lists', defaultSize: { w: 6, h: 2 }, single: true },
  { type: 'alerts.active', label: 'Active alerts list', description: 'The newest open alerts.', icon: 'list_alt', category: 'Lists', defaultSize: { w: 6, h: 2 }, single: true },

  {
    type: 'device.history',
    label: 'Device response history',
    description: 'Response time for devices you choose, over a range you choose.',
    icon: 'timeline',
    category: 'Devices and interfaces',
    defaultSize: { w: 6, h: 2 },
    configurable: true,
    defaultConfig: { deviceIds: [], range: '24h' },
  },
  {
    type: 'interface.throughput',
    label: 'Interface throughput',
    description: 'Inbound and outbound traffic on one SNMP interface.',
    icon: 'swap_vert',
    category: 'Devices and interfaces',
    defaultSize: { w: 6, h: 2 },
    configurable: true,
    defaultConfig: { range: '24h' },
  },
  {
    type: 'device.list',
    label: 'Device list',
    description: 'Devices filtered by location, status or tag, with their status.',
    icon: 'format_list_bulleted',
    category: 'Devices and interfaces',
    defaultSize: { w: 4, h: 2 },
    configurable: true,
    defaultConfig: { status: 'all', limit: 10 },
  },
];

export const WIDGET_CATEGORIES: readonly WidgetCategory[] = ['Summary', 'Charts', 'Lists', 'Devices and interfaces'];

const BY_TYPE = new Map(WIDGETS.map((w) => [w.type, w]));

export const widgetMeta = (type: string): WidgetMeta | undefined => BY_TYPE.get(type);

/** A widget's display title: its configured title, else the catalog label. */
export function widgetTitle(type: string, config?: Record<string, unknown>): string {
  const custom = typeof config?.title === 'string' ? config.title.trim() : '';
  return custom || widgetMeta(type)?.label || 'Unsupported widget';
}
