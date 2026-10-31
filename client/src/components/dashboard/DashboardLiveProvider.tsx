import { useState, type ReactNode } from 'react';
import type { Metric } from '../../api/types';
import { useSocket } from '../../hooks/useSocket';
import { HISTORY_LIMIT, LiveContext } from './useDashboardData';

/**
 * Collects metric samples pushed over the socket while a dashboard is open,
 * once for all of its widgets (the heatmap and expanded charts use them).
 */
export default function DashboardLiveProvider({ children }: { children: ReactNode }) {
  const [live, setLive] = useState<Metric[]>([]);
  useSocket({ onMetricUpdate: (metric) => setLive((prev) => [metric, ...prev].slice(0, HISTORY_LIMIT)) });
  return <LiveContext.Provider value={live}>{children}</LiveContext.Provider>;
}

