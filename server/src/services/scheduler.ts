import db from './database';
import { checkPing } from '../collectors/ping';
import { checkHttp } from '../collectors/http';
import { checkPort } from '../collectors/port';
import { collect as checkSystem } from '../collectors/system';
import { checkSnmp } from '../collectors/snmp';
import { evaluateAndCreateAlert } from './alertEngine';
import logger from './logger';

const jobs = new Map();

async function collectMetric(device: any) {
  switch (device.protocol) {
    case 'ping':
      return checkPing(device);
    case 'http':
      return checkHttp(device);
    case 'port':
      return checkPort(device);
    case 'system':
      return (checkSystem as any)(device);
    case 'snmp':
      return checkSnmp(device);
    default:
      return {
        status: 'down',
        responseTime: null,
        value: 0,
        message: `Unsupported protocol: ${device.protocol}`
      };
  }
}

async function runJob(io: any, device: any) {
  const sensor: any = db.getPrimarySensorForDevice(device.id);
  const metric: any = await collectMetric(device);
  db.recordMetric(device.id, metric.status, metric.responseTime, metric.value, metric.message, sensor?.id || null);

  const alert = evaluateAndCreateAlert(device, metric);

  const latest = {
    device_id: device.id,
    device_name: device.name,
    protocol: device.protocol,
    sensor_id: sensor?.id || null,
    sensor_name: sensor?.name || null,
    status: metric.status,
    response_time: metric.responseTime,
    value: metric.value,
    message: metric.message,
    timestamp: new Date().toISOString()
  };

  io.emit('metric:update', latest);
  if (alert) {
    io.emit('alert:triggered', alert);
  }
}

function clearJobs() {
  for (const intervalId of jobs.values()) {
    clearInterval(intervalId);
  }
  jobs.clear();
}

function startScheduler(io: any) {
  clearJobs();

  const devices: any[] = db.getDevices();
  for (const device of devices) {
    runJob(io, device).catch((error: any) => {
      logger.error({ device: device.name, err: error.message }, 'Initial job failed');
    });

    const intervalMs = Math.max(5, Number(device.interval_seconds || 60)) * 1000;
    const intervalId = setInterval(() => {
      runJob(io, device).catch((error: any) => {
        logger.error({ device: device.name, err: error.message }, 'Scheduled job failed');
      });
    }, intervalMs);

    jobs.set(device.id, intervalId);
  }
}

export { startScheduler, clearJobs };
