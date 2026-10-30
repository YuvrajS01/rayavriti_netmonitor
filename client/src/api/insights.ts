import { v1, unwrapGoResponse, type RequestOptions } from './http';
import type { InsightsResponse, HealthHistoryResponse } from './types';

export const getInsights = ({ signal }: RequestOptions = {}) =>
  v1.get('/insights/current', { signal }).then((r) => ({
    data: unwrapGoResponse<InsightsResponse>(r.data),
    success: true,
  }));

export const getInsightsHistory = (hours = 12, { signal }: RequestOptions = {}) =>
  v1.get(`/insights/history?hours=${hours}`, { signal }).then((r) => ({
    data: unwrapGoResponse<HealthHistoryResponse>(r.data),
    success: true,
  }));
