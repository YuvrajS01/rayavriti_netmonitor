import { v1, wrap, type RequestOptions } from './http';
import type { FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint } from './types';
import { formatBytes } from '../utils/formatters';

function buildQs(params: Record<string, string | number> = {}) {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return qs;
}

export const getFlowRecords = (params: Record<string, string | number> = {}, { signal }: RequestOptions = {}) =>
  v1.get(`/flows?${buildQs(params)}`, { signal }).then((r) => wrap<FlowRecord[]>(r.data));

export const getTopTalkers = (params: Record<string, string | number> = {}, { signal }: RequestOptions = {}) =>
  v1.get(`/flows/top-talkers?${buildQs(params)}`, { signal }).then((r) => wrap<TopTalker[]>(r.data));

/**
 * The backend returns either a ProtocolBreakdown[] or a legacy
 * `{ [protocol]: bytes }` map; normalise both to ProtocolBreakdown[].
 */
export const getProtocolDistribution = (params: Record<string, string> = {}, { signal }: RequestOptions = {}) =>
  v1.get(`/flows/protocols?${new URLSearchParams(params)}`, { signal }).then((r) => {
    const raw = wrap<Record<string, number> | ProtocolBreakdown[] | null>(r.data).data;
    if (!raw) return { data: [] as ProtocolBreakdown[], success: true };
    if (Array.isArray(raw)) return { data: raw, success: true };
    const entries = Object.entries(raw);
    const totalBytes = entries.reduce((s, [, v]) => s + v, 0);
    const data: ProtocolBreakdown[] = entries.map(([name, bytes]) => ({
      protocolName: name,
      protocolNumber: 0,
      bytes,
      bytesFormatted: formatBytes(bytes),
      packets: 0,
      flows: 0,
      percentage: totalBytes > 0 ? Math.round((bytes / totalBytes) * 100) : 0,
    }));
    return { data, success: true };
  });

export const getFlowTimeseries = (params: Record<string, string | number> = {}, { signal }: RequestOptions = {}) =>
  v1.get(`/flows/timeseries?${buildQs(params)}`, { signal }).then((r) => wrap<FlowTimeseriesPoint[]>(r.data));

/** Coerces the stats payload to numbers (the backend may send numeric strings). */
export const getFlowStats = ({ signal }: RequestOptions = {}) =>
  v1.get('/flows/stats', { signal }).then((r) => {
    const raw = wrap<Record<string, unknown> | null>(r.data).data;
    const data: FlowStats | null = raw
      ? {
          totalFlows: Number(raw.totalFlows) || 0,
          totalBytes: Number(raw.totalBytes) || 0,
          totalPackets: Number(raw.totalPackets) || 0,
          uniqueSources: Number(raw.uniqueSources) || 0,
          uniqueDestinations: Number(raw.uniqueDestinations) || 0,
        }
      : null;
    return { data, success: true };
  });
