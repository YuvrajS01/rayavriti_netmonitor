import { v1, wrapJson, type RequestOptions } from './http';
import type { InterfaceHistoryPoint, InterfaceSample, LLDPNeighbor } from './types';

/**
 * Latest sample of each interface seen in the last hour, or in `since` (a
 * Go duration up to 168h) to include interfaces of a device that is
 * briefly unreachable.
 */
export const getDeviceInterfaces = (id: number, { signal, since }: RequestOptions & { since?: string } = {}) =>
  v1.get(`/devices/${id}/interfaces${since ? `?since=${since}` : ''}`, { signal }).then((r) => wrapJson<InterfaceSample[] | null>(r.data).data ?? []);

/** LLDP neighbours from the device's last neighbour walk. */
export const getDeviceNeighbors = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/neighbors`, { signal }).then((r) => wrapJson<LLDPNeighbor[] | null>(r.data).data ?? []);

/** Bucketed throughput for one interface; the server picks the bucket from the span. */
export const getInterfaceHistory = (id: number, ifIndex: number, from: string, to: string, { signal }: RequestOptions = {}) =>
  v1
    .get(`/devices/${id}/interfaces/${ifIndex}/history?${new URLSearchParams({ from, to })}`, { signal })
    .then((r) => wrapJson<InterfaceHistoryPoint[] | null>(r.data).data ?? []);
