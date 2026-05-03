const db = require('./database');
const { checkPing } = require('../collectors/ping');
const { checkHttp } = require('../collectors/http');
const { checkPort } = require('../collectors/port');
const { collect: checkSystem } = require('../collectors/system');
const { checkSnmp } = require('../collectors/snmp');
const { evaluateAndCreateAlert } = require('./alertEngine');

const jobs = new Map();

async function collectMetric(device) {
  switch (device.protocol) {
    case 'ping':
      return checkPing(device);
    case 'http':
      return checkHttp(device);
    case 'port':
      return checkPort(device);
    case 'system':
      return checkSystem(device);
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

async function runJob(io, device) {
  const sensor = db.getPrimarySensorForDevice(device.id);
  const metric = await collectMetric(device);
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

function startScheduler(io) {
  clearJobs();

  const devices = db.getDevices();
  for (const device of devices) {
    runJob(io, device).catch((error) => {
      console.error(`Initial job failed for ${device.name}:`, error.message);
    });

    const intervalMs = Math.max(5, Number(device.interval_seconds || 60)) * 1000;
    const intervalId = setInterval(() => {
      runJob(io, device).catch((error) => {
        console.error(`Job failed for ${device.name}:`, error.message);
      });
    }, intervalMs);

    jobs.set(device.id, intervalId);
  }
}

module.exports = { startScheduler, clearJobs };
