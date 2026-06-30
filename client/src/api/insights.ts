import { v1, unwrapGoResponse } from './http';
import type { InsightsResponse, HealthHistoryResponse } from './types';

export const getInsights = () =>
  v1.get('/insights/current').then((r) => ({
    data: unwrapGoResponse(r.data) as InsightsResponse,
    success: true,
  }));

export const getInsightsHistory = (hours = 12) =>
  v1.get(`/insights/history?hours=${hours}`).then((r) => ({
    data: unwrapGoResponse(r.data) as HealthHistoryResponse,
    success: true,
  }));
