import type { ComponentType } from 'react';
import { lazyWithRetry } from '../../../utils/lazyWithRetry';
import type { WidgetProps } from './ConfigurableWidgets';
import { UnsupportedWidget } from './WidgetCard';

type WidgetComponent = ComponentType<WidgetProps>;

const overview = () => import('./OverviewWidgets');
const configurable = () => import('./ConfigurableWidgets');

/** A lazily loaded named export (the overview widgets share one chunk). */
function named<M>(load: () => Promise<M>, pick: (m: M) => ComponentType<never>): WidgetComponent {
  return lazyWithRetry(() => load().then((m) => ({ default: pick(m) as unknown as WidgetComponent })));
}

const COMPONENTS: Record<string, WidgetComponent> = {
  'stat.total-devices': named(overview, (m) => m.TotalDevicesWidget),
  'stat.online': named(overview, (m) => m.OnlineWidget),
  'stat.uptime': named(overview, (m) => m.UptimeWidget),
  'stat.active-alerts': named(overview, (m) => m.ActiveAlertsStatWidget),
  'health.score': named(overview, (m) => m.HealthScoreWidget),
  insights: named(overview, (m) => m.InsightsWidget),
  'response.slowest': named(overview, (m) => m.SlowestResponseWidget),
  'status.distribution': named(overview, (m) => m.StatusDistributionWidget),
  'status.heatmap': named(overview, (m) => m.StatusHeatmapWidget),
  'topology.mini': named(overview, (m) => m.TopologyMiniWidget),
  'resource.load': named(overview, (m) => m.ResourceLoadWidget),
  'response.by-status': named(overview, (m) => m.ResponseByStatusWidget),
  'metrics.latest': named(overview, (m) => m.LatestMetricsWidget),
  'alerts.active': named(overview, (m) => m.ActiveAlertsListWidget),
  'device.history': named(configurable, (m) => m.DeviceHistoryWidget),
  'interface.throughput': named(configurable, (m) => m.InterfaceThroughputWidget),
  'device.list': named(configurable, (m) => m.DeviceListWidget),
};


export function widgetComponent(type: string): WidgetComponent {
  return COMPONENTS[type] ?? (UnsupportedWidget as WidgetComponent);
}

export const KNOWN_WIDGET_TYPES = Object.keys(COMPONENTS);
