import db from './database';

function evaluateAndCreateAlert(device: any, metric: any) {
  if (!device || !metric) {
    return null;
  }

  let severity = null;
  let message = null;

  if (metric.status === 'down') {
    severity = 'critical';
    message = `${device.name} is down (${device.protocol})`;
  } else if (metric.status === 'degraded' || metric.status === 'warning') {
    severity = 'warning';
    message = `${device.name} degraded: ${metric.message}`;
  } else if (typeof metric.responseTime === 'number' && metric.responseTime > 500) {
    severity = 'warning';
    message = `${device.name} latency high (${metric.responseTime}ms)`;
  }

  if (!severity || !message) {
    return null;
  }

  const existing = db
    .getActiveAlerts()
    .find((a: any) => a.device_id === device.id && a.message === message);

  if (existing) {
    return null;
  }

  const result = db.createAlert(device.id, severity, message);
  return {
    id: result.lastInsertRowid,
    deviceId: device.id,
    deviceName: device.name,
    severity,
    message,
    status: 'active',
    createdAt: new Date().toISOString()
  };
}

export { evaluateAndCreateAlert };
