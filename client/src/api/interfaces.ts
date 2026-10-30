import { v1, wrapJson, type RequestOptions } from './http';
import type { InterfaceSample, LLDPNeighbor } from './types';

/** Latest sample of each interface seen in the last hour. */
export const getDeviceInterfaces = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/interfaces`, { signal }).then((r) => wrapJson<InterfaceSample[] | null>(r.data).data ?? []);

/** LLDP neighbours from the device's last neighbour walk. */
export const getDeviceNeighbors = (id: number, { signal }: RequestOptions = {}) =>
  v1.get(`/devices/${id}/neighbors`, { signal }).then((r) => wrapJson<LLDPNeighbor[] | null>(r.data).data ?? []);
