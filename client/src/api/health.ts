import { v1, unwrapGoResponse } from './http';
import type { DeviceHealthScore } from './types';

export const getHealthScores = () =>
  v1.get('/health/scores').then((r) => ({
    data: unwrapGoResponse(r.data) as DeviceHealthScore[],
    success: true,
  }));

export const getHealthScore = (deviceId: number) =>
  v1.get(`/health/scores/${deviceId}`).then((r) => ({
    data: unwrapGoResponse(r.data) as DeviceHealthScore,
    success: true,
  }));
