const db = require('./database');
const flowAnalyzer = require('./flowAnalyzer');

function toSqlDate(date) {
  return date.toISOString().slice(0, 19).replace('T', ' ');
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function average(values) {
  if (!values.length) {
    return 0;
  }
  return values.reduce((sum, x) => sum + x, 0) / values.length;
}

function stddev(values) {
  if (values.length < 2) {
    return 0;
  }
  const avg = average(values);
  const variance = average(values.map((x) => Math.pow(x - avg, 2)));
  return Math.sqrt(variance);
}

function scoreLabel(score) {
  if (score >= 85) return 'healthy';
  if (score >= 65) return 'watch';
  if (score >= 40) return 'risk';
  return 'critical';
}

// ── Factor weights ────────────────────────────────────────────
const FACTOR_WEIGHTS = {
  availability: 0.35,
  latency: 0.25,
  alerts: 0.20,
  stability: 0.12,
  ports: 0.08,
};

function getMetricsWindow(hours = 24) {
  const since = toSqlDate(new Date(Date.now() - hours * 60 * 60 * 1000));
  return db.getRecentMetrics({ since, limit: 10000 });
}

function detectResponseAnomalies(metrics) {
  const byDevice = new Map();
  for (const m of metrics) {
    if (!byDevice.has(m.device_id)) {
      byDevice.set(m.device_id, []);
    }
    byDevice.get(m.device_id).push(m);
  }

  const anomalies = [];
  for (const [deviceId, rows] of byDevice.entries()) {
    const ordered = [...rows].sort((a, b) => String(a.timestamp).localeCompare(String(b.timestamp)));
    const recent = ordered.slice(-5);
    const baseline = ordered.slice(0, -5).map((m) => Number(m.response_time)).filter((n) => Number.isFinite(n) && n > 0);
    const recentValues = recent.map((m) => Number(m.response_time)).filter((n) => Number.isFinite(n) && n > 0);

    if (baseline.length < 5 || recentValues.length === 0) {
      continue;
    }

    const avg = average(baseline);
    const sd = Math.max(stddev(baseline), 25);
    const latest = recent[recent.length - 1];
    const latestValue = Number(latest.response_time || 0);

    if (latestValue > avg + sd * 2.5 && latestValue > avg * 1.8) {
      anomalies.push({
        type: 'response_time_anomaly',
        severity: latestValue > avg + sd * 4 ? 'critical' : 'warning',
        deviceId,
        deviceName: latest.device_name,
        message: `${latest.device_name} latency is unusual: ${Math.round(latestValue)}ms vs ${Math.round(avg)}ms baseline`,
        observed: Math.round(latestValue),
        baseline: Math.round(avg),
        timestamp: latest.timestamp
      });
    }
  }

  return anomalies;
}

function groupAlerts(alerts) {
  const groups = new Map();
  const recentAlerts = alerts.filter((a) => a.status === 'active');

  for (const alert of recentAlerts) {
    const key = `${alert.device_id}:${alert.severity}`;
    if (!groups.has(key)) {
      groups.set(key, {
        deviceId: alert.device_id,
        deviceName: alert.device_name,
        severity: alert.severity,
        count: 0,
        messages: [],
        firstSeen: alert.created_at,
        lastSeen: alert.created_at
      });
    }

    const group = groups.get(key);
    group.count += 1;
    group.messages.push(alert.message);
    if (String(alert.created_at) < String(group.firstSeen)) group.firstSeen = alert.created_at;
    if (String(alert.created_at) > String(group.lastSeen)) group.lastSeen = alert.created_at;
  }

  return Array.from(groups.values()).map((group) => ({
    ...group,
    summary: `${group.deviceName || `Device ${group.deviceId}`} has ${group.count} active ${group.severity} alert${group.count === 1 ? '' : 's'}`
  })).sort((a, b) => b.count - a.count);
}

/**
 * Compute per-device health with weighted factor breakdown.
 */
function computeDeviceHealth(metrics, portResults, alerts) {
  const byDevice = new Map();
  for (const m of metrics) {
    if (!byDevice.has(m.device_id)) {
      byDevice.set(m.device_id, []);
    }
    byDevice.get(m.device_id).push(m);
  }

  const activeAlertsByDevice = new Map();
  for (const alert of alerts.filter((a) => a.status === 'active')) {
    activeAlertsByDevice.set(alert.device_id, (activeAlertsByDevice.get(alert.device_id) || 0) + 1);
  }

  return Array.from(byDevice.entries()).map(([deviceId, rows]) => {
    const samples = rows.length;
    const down = rows.filter((m) => m.status === 'down').length;
    const warning = rows.filter((m) => m.status === 'warning' || m.status === 'degraded').length;
    const responseValues = rows.map((m) => Number(m.response_time)).filter((n) => Number.isFinite(n) && n > 0);
    const avgResponse = average(responseValues);
    const alertCount = activeAlertsByDevice.get(deviceId) || 0;
    const devicePorts = portResults.filter((p) => p.device_id === deviceId);
    const changedPorts = devicePorts.filter((p) => p.last_changed_at && p.last_changed_at === p.last_seen).length;

    // ── Individual factor scores (0–100 each) ──
    const availabilityRaw = samples ? ((samples - down) / samples) * 100 : 75;
    const availabilityScore = Math.round(clamp(availabilityRaw, 0, 100));

    const warningRatio = samples ? warning / samples : 0;
    const stabilityScore = Math.round(clamp(100 - warningRatio * 180, 0, 100));

    const latencyScore = Math.round(clamp(100 - avgResponse / 4, 0, 100));

    const alertScore = Math.round(clamp(100 - alertCount * 20, 0, 100));

    const portScore = Math.round(clamp(100 - changedPorts * 15, 0, 100));

    // ── Weighted composite ──
    const compositeScore = Math.round(
      availabilityScore * FACTOR_WEIGHTS.availability +
      latencyScore * FACTOR_WEIGHTS.latency +
      alertScore * FACTOR_WEIGHTS.alerts +
      stabilityScore * FACTOR_WEIGHTS.stability +
      portScore * FACTOR_WEIGHTS.ports
    );
    const score = clamp(compositeScore, 0, 100);

    const factors = {
      availability: { score: availabilityScore, weight: FACTOR_WEIGHTS.availability, penalty: 100 - availabilityScore },
      latency:      { score: latencyScore, weight: FACTOR_WEIGHTS.latency, penalty: 100 - latencyScore },
      alerts:       { score: alertScore, weight: FACTOR_WEIGHTS.alerts, penalty: 100 - alertScore },
      stability:    { score: stabilityScore, weight: FACTOR_WEIGHTS.stability, penalty: 100 - stabilityScore },
      ports:        { score: portScore, weight: FACTOR_WEIGHTS.ports, penalty: 100 - portScore },
    };

    const latest = rows.sort((a, b) => String(b.timestamp).localeCompare(String(a.timestamp)))[0];
    const issues = [];

    if (down > 0) {
      issues.push({
        severity: 'critical',
        type: 'availability',
        message: `${down} failed check${down === 1 ? '' : 's'} in the last 24 hours`
      });
    }

    if (warning > 0) {
      issues.push({
        severity: 'warning',
        type: 'warning_samples',
        message: `${warning} degraded sample${warning === 1 ? '' : 's'} detected`
      });
    }

    if (avgResponse > 500) {
      issues.push({
        severity: avgResponse > 1000 ? 'critical' : 'warning',
        type: 'latency',
        message: `Average response time is ${Math.round(avgResponse)}ms`
      });
    }

    if (alertCount > 0) {
      issues.push({
        severity: alertCount > 2 ? 'critical' : 'warning',
        type: 'active_alerts',
        message: `${alertCount} active alert${alertCount === 1 ? '' : 's'}`
      });
    }

    if (changedPorts > 0) {
      issues.push({
        severity: 'info',
        type: 'port_changes',
        message: `${changedPorts} port state change${changedPorts === 1 ? '' : 's'} observed`
      });
    }

    if (issues.length === 0) {
      issues.push({
        severity: 'info',
        type: 'clear',
        message: 'No active issues detected from recent telemetry'
      });
    }

    return {
      deviceId,
      deviceName: latest?.device_name || `Device ${deviceId}`,
      score,
      label: scoreLabel(score),
      availabilityPercent: Number(availabilityRaw.toFixed(1)),
      avgResponseMs: Math.round(avgResponse),
      activeAlerts: alertCount,
      openPorts: devicePorts.filter((p) => p.status === 'open').length,
      samples,
      factors,
      // trend/trendDelta added by addTrendData()
      trend: 'stable' as 'improving' | 'stable' | 'degrading',
      trendDelta: 0,
      issues
    };
  }).sort((a, b) => a.score - b.score);
}

/**
 * Compute a lightweight health score for a set of metrics (used for history timeline).
 * Returns the average composite score across all devices in the window.
 */
function computeWindowScore(metrics, portResults, alerts) {
  if (!metrics.length) return null;
  const health = computeDeviceHealth(metrics, portResults, alerts);
  if (!health.length) return null;
  return Math.round(health.reduce((sum, d) => sum + d.score, 0) / health.length);
}

/**
 * Compare current health against a baseline window to detect trends.
 */
function addTrendData(healthList, baselineMetrics, portResults, alerts) {
  if (!baselineMetrics.length) return healthList;

  const baselineHealth = computeDeviceHealth(baselineMetrics, portResults, alerts);
  const baselineMap = new Map();
  for (const bh of baselineHealth) {
    baselineMap.set(bh.deviceId, bh.score);
  }

  for (const device of healthList) {
    const baselineScore = baselineMap.get(device.deviceId);
    if (baselineScore !== undefined) {
      const delta = device.score - baselineScore;
      device.trendDelta = delta;
      if (delta >= 5) {
        device.trend = 'improving';
      } else if (delta <= -5) {
        device.trend = 'degrading';
      } else {
        device.trend = 'stable';
      }
    }
  }

  return healthList;
}

function buildInsights() {
  const metrics = getMetricsWindow(24);
  const alerts = db.getAlerts({ status: 'active', limit: 500 });
  const devices = db.getDevices();
  const portResults = devices.flatMap((d) => db.getPortScanResults(d.id));

  const responseAnomalies = detectResponseAnomalies(metrics);
  const flowAnomalies = flowAnalyzer.detectAnomalies();
  const alertGroups = groupAlerts(alerts);
  let health = computeDeviceHealth(metrics, portResults, alerts);

  // ── Trend detection ──
  try {
    const trendData = db.getMetricsForTrend(1);
    if (trendData.baseline.length > 0) {
      health = addTrendData(health, trendData.baseline, portResults, alerts);
    }
  } catch {
    // trend data unavailable, continue without
  }

  // ── Network-wide summary ──
  const networkScore = health.length
    ? Math.round(health.reduce((sum, d) => sum + d.score, 0) / health.length)
    : 0;

  const healthDistribution = {
    critical: health.filter((d) => d.label === 'critical').length,
    risk: health.filter((d) => d.label === 'risk').length,
    watch: health.filter((d) => d.label === 'watch').length,
    healthy: health.filter((d) => d.label === 'healthy').length,
  };

  const topRisks = [...health]
    .filter((d) => d.score < 85)
    .sort((a, b) => a.trendDelta - b.trendDelta)
    .slice(0, 3)
    .map((d) => ({
      deviceId: d.deviceId,
      deviceName: d.deviceName,
      score: d.score,
      label: d.label,
      trend: d.trend,
      trendDelta: d.trendDelta,
      primaryIssue: (d.issues.find((i) => i.severity !== 'info') || d.issues[0])?.message || 'No issue'
    }));

  const insights = [];
  for (const anomaly of responseAnomalies.slice(0, 5)) {
    insights.push({
      type: anomaly.type,
      severity: anomaly.severity,
      title: 'Latency anomaly',
      message: anomaly.message,
      deviceId: anomaly.deviceId,
      timestamp: anomaly.timestamp
    });
  }

  for (const group of alertGroups.slice(0, 4)) {
    insights.push({
      type: 'alert_group',
      severity: group.severity,
      title: 'Grouped alert',
      message: group.summary,
      deviceId: group.deviceId,
      timestamp: group.lastSeen
    });
  }

  for (const anomaly of flowAnomalies.slice(0, 4)) {
    insights.push({
      type: anomaly.type,
      severity: anomaly.severity,
      title: anomaly.type === 'traffic_spike' ? 'Traffic spike' : 'Flow anomaly',
      message: anomaly.message,
      timestamp: anomaly.timestamp
    });
  }

  for (const item of health.filter((h) => h.score < 70).slice(0, 4)) {
    insights.push({
      type: 'health_score',
      severity: item.score < 40 ? 'critical' : 'warning',
      title: 'Health risk',
      message: `${item.deviceName} health score is ${item.score}/100 (${item.label})`,
      deviceId: item.deviceId,
      timestamp: new Date().toISOString()
    });
  }

  return {
    generatedAt: new Date().toISOString(),
    networkScore,
    healthDistribution,
    topRisks,
    health,
    responseAnomalies,
    alertGroups,
    flowAnomalies,
    insights: insights.sort((a, b) => {
      const severityRank = { critical: 0, warning: 1, info: 2 };
      return (severityRank[a.severity] ?? 3) - (severityRank[b.severity] ?? 3);
    })
  };
}

/**
 * Build a 12-hour health timeline with hourly data points.
 */
function buildHistoryTimeline(hours = 12) {
  const alerts = db.getAlerts({ status: 'all', limit: 2000 });
  const devices = db.getDevices();
  const portResults = devices.flatMap((d) => db.getPortScanResults(d.id));
  const now = Date.now();
  const points = [];

  for (let i = hours; i >= 0; i--) {
    const windowEnd = new Date(now - i * 60 * 60 * 1000);
    const windowStart = new Date(windowEnd.getTime() - 60 * 60 * 1000);
    const fromIso = toSqlDate(windowStart);
    const toIso = toSqlDate(windowEnd);

    try {
      const windowMetrics = db.getMetricsInWindow(fromIso, toIso);
      const score = computeWindowScore(windowMetrics, portResults, alerts);
      points.push({
        timestamp: windowEnd.toISOString(),
        score: score !== null ? score : null,
        label: score !== null ? scoreLabel(score) : null,
      });
    } catch {
      points.push({
        timestamp: windowEnd.toISOString(),
        score: null,
        label: null,
      });
    }
  }

  return {
    generatedAt: new Date().toISOString(),
    hours,
    points,
  };
}

module.exports = {
  buildInsights,
  buildHistoryTimeline,
  detectResponseAnomalies,
  groupAlerts,
  computeDeviceHealth,
  // Called per-metric by the simulator endpoint — no-op for now,
  // real-time anomaly detection is handled at query time by buildInsights()
  processMetric: () => {}
};

export {};
