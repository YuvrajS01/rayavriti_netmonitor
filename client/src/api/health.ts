import { v1, unwrapGoResponse, type RequestOptions } from './http';
import type { DeviceHealthScore } from './types';

export const getHealthScores = ({ signal }: RequestOptions = {}) =>
  v1.get('/health/scores', { signal }).then((r) => ({
    data: unwrapGoResponse<DeviceHealthScore[]>(r.data),
    success: true,
  }));

export const getHealthScore = (deviceId: number, { signal }: RequestOptions = {}) =>
  v1.get(`/health/scores/${deviceId}`, { signal }).then((r) => ({
    data: unwrapGoResponse<DeviceHealthScore>(r.data),
    success: true,
  }));
