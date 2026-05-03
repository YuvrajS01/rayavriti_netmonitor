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

    const availabilityScore = samples ? ((samples - down) / samples) * 100 : 75;
    const warningPenalty = samples ? (warning / samples) * 18 : 0;
    const latencyPenalty = clamp(avgResponse / 40, 0, 22);
    const alertPenalty = clamp(alertCount * 8, 0, 28);
    const portPenalty = clamp(changedPorts * 3, 0, 12);
    const score = Math.round(clamp(availabilityScore - warningPenalty - latencyPenalty - alertPenalty - portPenalty, 0, 100));
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
      availabilityPercent: Number(availabilityScore.toFixed(1)),
      avgResponseMs: Math.round(avgResponse),
      activeAlerts: alertCount,
      openPorts: devicePorts.filter((p) => p.status === 'open').length,
      samples,
      issues
    };
  }).sort((a, b) => a.score - b.score);
}

function buildInsights() {
  const metrics = getMetricsWindow(24);
  const alerts = db.getAlerts({ status: 'active', limit: 500 });
  const devices = db.getDevices();
  const portResults = devices.flatMap((d) => db.getPortScanResults(d.id));

  const responseAnomalies = detectResponseAnomalies(metrics);
  const flowAnomalies = flowAnalyzer.detectAnomalies();
  const alertGroups = groupAlerts(alerts);
  const health = computeDeviceHealth(metrics, portResults, alerts);

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

module.exports = {
  buildInsights,
  detectResponseAnomalies,
  groupAlerts,
  computeDeviceHealth
};

export {};
