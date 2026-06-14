import { api, unwrapGoResponse } from './http';
import type { InsightsResponse, HealthHistoryResponse } from './types';

export type MetricsForInsights = Array<{ deviceId: number; responseTime: number | null; status: string; timestamp: string }>;
export type AlertsForInsights = Array<{ deviceId: number }>;

export const getInsights = (prefetched?: { metrics?: MetricsForInsights; alerts?: AlertsForInsights }) =>
  Promise.all([
    api.get('/insights').then((r) => unwrapGoResponse(r.data)),
    prefetched?.metrics
      ? Promise.resolve(prefetched.metrics)
      : api.get('/metrics/latest').then((r) => {
          const data = unwrapGoResponse(r.data);
          return data as MetricsForInsights;
        }).catch(() => [] as MetricsForInsights),
    prefetched?.alerts
      ? Promise.resolve(prefetched.alerts)
      : api.get('/alerts?status=active').then((r) => {
          const data = unwrapGoResponse(r.data);
          if (data && typeof data === 'object' && 'alerts' in data && 'total' in data) {
            return (data as Record<string, unknown>).alerts as AlertsForInsights;
          }
          return data as AlertsForInsights;
        }).catch(() => [] as AlertsForInsights),
  ]).then(([inner, metrics, alerts]) => {
    if (!Array.isArray(inner)) {
      return { data: inner, success: true } as { data: InsightsResponse; success: boolean };
    }

    const items = inner as Array<{ deviceId: number; deviceName: string; score: number; status: string }>;

    const metricByDevice = new Map<number, { responseTime: number; status: string }>();
    for (const m of metrics) {
      const existing = metricByDevice.get(m.deviceId);
      if (!existing || new Date(m.timestamp) > new Date(existing.status)) {
        metricByDevice.set(m.deviceId, { responseTime: m.responseTime ?? 0, status: m.status });
      }
    }

    const alertsByDevice = new Map<number, number>();
    for (const a of alerts) {
      alertsByDevice.set(a.deviceId, (alertsByDevice.get(a.deviceId) || 0) + 1);
    }

    const avgScore = items.length ? Math.round(items.reduce((s, d) => s + d.score, 0) / items.length) : 0;
    const critical = items.filter((d) => d.score < 40).length;
    const watch = items.filter((d) => d.score >= 40 && d.score < 70).length;
    const healthy = items.filter((d) => d.score >= 70).length;

    const health = items.map((d) => {
      const metric = metricByDevice.get(d.deviceId);
      const deviceAlerts = alertsByDevice.get(d.deviceId) || 0;
      const isUp = d.status === 'up' || d.status === 'ok';
      const isDown = d.status === 'down';
      const rt = metric?.responseTime ?? 0;

      const availabilityScore = isUp ? 100 : isDown ? 0 : 50;
      const latencyScore = rt === 0 ? 80 : rt < 200 ? 100 : rt < 500 ? 85 : rt < 1000 ? 65 : rt < 2000 ? 40 : 15;
      const alertScore = deviceAlerts === 0 ? 100 : deviceAlerts === 1 ? 70 : deviceAlerts <= 3 ? 40 : 10;
      const stabilityScore = isUp ? 95 : isDown ? 20 : 50;
      const portScore = 80;

      const issues: Array<{ severity: 'critical' | 'warning' | 'info'; type: string; message: string }> = [];
      if (isDown) issues.push({ severity: 'critical', type: 'availability', message: 'Device is offline' });
      if (rt > 1000) issues.push({ severity: 'warning', type: 'latency', message: `High latency: ${Math.round(rt)}ms` });
      if (deviceAlerts > 0) issues.push({ severity: deviceAlerts > 2 ? 'critical' : 'warning', type: 'alerts', message: `${deviceAlerts} active alert${deviceAlerts > 1 ? 's' : ''}` });
      if (d.status === 'warning' || d.status === 'degraded') issues.push({ severity: 'warning', type: 'status', message: 'Device reporting warnings' });

      return {
        deviceId: d.deviceId,
        deviceName: d.deviceName,
        score: d.score,
        label: (d.score < 40 ? 'critical' : d.score < 60 ? 'risk' : d.score < 80 ? 'watch' : 'healthy') as 'critical' | 'risk' | 'watch' | 'healthy',
        availabilityPercent: availabilityScore,
        avgResponseMs: Math.round(rt),
        activeAlerts: deviceAlerts,
        openPorts: 0,
        samples: 1,
        factors: {
          availability: { score: availabilityScore, weight: 0.3, penalty: 100 - availabilityScore },
          latency: { score: latencyScore, weight: 0.25, penalty: 100 - latencyScore },
          alerts: { score: alertScore, weight: 0.2, penalty: 100 - alertScore },
          stability: { score: stabilityScore, weight: 0.15, penalty: 100 - stabilityScore },
          ports: { score: portScore, weight: 0.1, penalty: 100 - portScore },
        },
        trend: 'stable' as const,
        trendDelta: 0,
        issues,
      };
    });

    return {
      data: {
        generatedAt: new Date().toISOString(),
        networkScore: avgScore,
        healthDistribution: { critical, risk: 0, watch, healthy },
        topRisks: items.filter((d) => d.score < 70).slice(0, 5).map((d) => ({
          deviceId: d.deviceId, deviceName: d.deviceName, score: d.score,
          label: d.score < 40 ? 'critical' : 'risk',
          trend: 'stable' as const, trendDelta: 0, primaryIssue: d.status !== 'up' ? `Status: ${d.status}` : 'No issues',
        })),
        health,
        insights: items.filter((d) => d.score < 70).map((d) => ({
          deviceId: d.deviceId, deviceName: d.deviceName, score: d.score, status: d.status,
          type: 'health', severity: d.score < 40 ? 'critical' as const : 'warning' as const,
          title: `${d.deviceName} — ${d.score}%`, message: d.status !== 'up' ? `Device is ${d.status}` : `Score: ${d.score}%`,
        })),
      },
      success: true,
    };
  });

export const getInsightsHistory = (hours?: number) => {
  const qs = hours ? `?hours=${hours}` : '';
  return api.get(`/insights/history${qs}`).then((r) => {
    const data = unwrapGoResponse(r.data);
    return { data: data as HealthHistoryResponse, success: true };
  });
};
