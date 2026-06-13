import { v1, wrap } from './http';
import type { FlowRecord, TopTalker, ProtocolBreakdown, FlowStats, FlowTimeseriesPoint } from './types';

function buildQs(params: Record<string, string | number> = {}) {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) { if (v) qs.set(k, String(v)); }
  return qs;
}

export const getFlowRecords = (params: Record<string, string | number> = {}) =>
  v1.get(`/flows?${buildQs(params)}`).then((r) => wrap<FlowRecord[]>(r.data));

export const getTopTalkers = (params: Record<string, string | number> = {}) =>
  v1.get(`/flows/top-talkers?${buildQs(params)}`).then((r) => wrap<TopTalker[]>(r.data));

export const getProtocolDistribution = (params: Record<string, string> = {}) =>
  v1.get(`/flows/protocols?${new URLSearchParams(params)}`).then((r) => wrap<ProtocolBreakdown[]>(r.data));

export const getFlowTimeseries = (params: Record<string, string | number> = {}) =>
  v1.get(`/flows/timeseries?${buildQs(params)}`).then((r) => wrap<FlowTimeseriesPoint[]>(r.data));

export const getFlowStats = () =>
  v1.get('/flows/stats').then((r) => wrap<FlowStats>(r.data));
