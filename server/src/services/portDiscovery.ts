const db = require('./database');
const { scanPorts } = require('../collectors/portScanner');

const RISKY_PORTS = new Map([
  [23, 'Telnet'],
  [445, 'SMB'],
  [3389, 'RDP'],
  [5900, 'VNC'],
  [2375, 'Docker API'],
  [6379, 'Redis'],
  [11211, 'Memcached']
]);

async function scanDevicePorts(device, options = {}) {
  const before = new Map<number, any>(db.getPortScanResults(device.id).map((row) => [Number(row.port), row]));
  const results = await scanPorts(device.host, options);
  const changes = [];
  const alerts = [];

  for (const result of results) {
    const previous = before.get(result.port);
    db.upsertPortScanResult({
      deviceId: device.id,
      port: result.port,
      status: result.status,
      serviceGuess: result.serviceGuess,
      responseTime: result.responseTime
    });

    if (previous && previous.status !== result.status) {
      changes.push({
        port: result.port,
        from: previous.status,
        to: result.status,
        serviceGuess: result.serviceGuess
      });
    } else if (!previous && result.status === 'open') {
      changes.push({
        port: result.port,
        from: 'unknown',
        to: 'open',
        serviceGuess: result.serviceGuess
      });
    }

    if (result.status === 'open' && RISKY_PORTS.has(result.port)) {
      const service = RISKY_PORTS.get(result.port);
      const message = `${device.name} has risky port ${result.port} open (${service})`;
      if (!db.findActiveAlert(device.id, message)) {
        const created = db.createAlert(device.id, 'warning', message);
        alerts.push({
          id: created.lastInsertRowid,
          deviceId: device.id,
          deviceName: device.name,
          severity: 'warning',
          message,
          status: 'active',
          createdAt: new Date().toISOString()
        });
      }
    }
  }

  return {
    deviceId: device.id,
    host: device.host,
    scannedPorts: results.length,
    openPorts: results.filter((r) => r.status === 'open'),
    results,
    changes,
    alerts
  };
}

module.exports = {
  scanDevicePorts
};

export {};
