const crypto = require('crypto');

const express = require('express');
const http = require('http');
const { Server } = require('socket.io');

const db = require('./services/database');
const auth = require('./services/auth');
const { startScheduler, clearJobs } = require('./services/scheduler');
const { startNetflowCollector, stopNetflowCollector } = require('./collectors/netflow');
const {
  listInterfaces, startCapture, stopCapture,
  getSessionPackets, getSessionStats, stopAllCaptures
} = require('./collectors/packetCapture');
const flowAnalyzer = require('./services/flowAnalyzer');

const app = express();
const server = http.createServer(app);
const io = new Server(server);

const PORT = Number(process.env.PORT || 3000);

const rateCounters = new Map();

function toSqlDate(date) {
  return date.toISOString().slice(0, 19).replace('T', ' ');
}

function parseReportRange(req) {
  const toDate = req.query.to ? new Date(req.query.to) : new Date();
  const fromDate = req.query.from
    ? new Date(req.query.from)
    : new Date(toDate.getTime() - 24 * 60 * 60 * 1000);

  if (Number.isNaN(fromDate.getTime()) || Number.isNaN(toDate.getTime())) {
    return null;
  }

  return {
    from: toSqlDate(fromDate),
    to: toSqlDate(toDate)
  };
}

function toCsv(rows) {
  const header = [
    'id',
    'device_id',
    'device_name',
    'protocol',
    'status',
    'response_time',
    'value',
    'message',
    'timestamp'
  ];

  const escape = (value) => {
    const text = String(value ?? '');
    if (text.includes(',') || text.includes('"') || text.includes('\n')) {
      return `"${text.replace(/"/g, '""')}"`;
    }
    return text;
  };

  const lines = rows.map((row) => header.map((key) => escape(row[key])).join(','));
  return [header.join(','), ...lines].join('\n');
}

function getReqId(req) {
  return req.requestId || crypto.randomBytes(8).toString('hex');
}

function sendOk(req, res, data, extraMeta = {}, status = 200) {
  const payload = {
    success: true,
    data,
    error: null,
    meta: {
      timestamp: new Date().toISOString(),
      requestId: getReqId(req),
      pagination: extraMeta.pagination || null,
      ...extraMeta
    }
  };

  if (!extraMeta.pagination) {
    payload.meta.pagination = null;
  }

  return res.status(status).json(payload);
}

function sendError(req, res, status, code, message) {
  return res.status(status).json({
    success: false,
    data: null,
    error: {
      code,
      message
    },
    meta: {
      timestamp: new Date().toISOString(),
      requestId: getReqId(req),
      pagination: null
    }
  });
}

function sanitizeDevice(device) {
  if (!device) {
    return device;
  }
  const { snmp_community, ...rest } = device;
  return rest;
}

function parsePagination(query) {
  const page = Math.max(1, Number(query.page || 1));
  const pageSize = Math.max(1, Math.min(200, Number(query.pageSize || 20)));
  return { page, pageSize };
}

function applySort(items, sortRaw, fieldMap) {
  if (!sortRaw) {
    return items;
  }

  const desc = sortRaw.startsWith('-');
  const key = desc ? sortRaw.slice(1) : sortRaw;
  const field = fieldMap[key];
  if (!field) {
    return items;
  }

  return [...items].sort((a, b) => {
    const left = a[field];
    const right = b[field];
    if (left === right) {
      return 0;
    }
    if (left === undefined || left === null) {
      return desc ? 1 : -1;
    }
    if (right === undefined || right === null) {
      return desc ? -1 : 1;
    }

    if (typeof left === 'number' && typeof right === 'number') {
      return desc ? right - left : left - right;
    }

    const cmp = String(left).localeCompare(String(right));
    return desc ? -cmp : cmp;
  });
}

function paginate(items, page, pageSize) {
  const total = items.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(page, totalPages);
  const start = (safePage - 1) * pageSize;
  const data = items.slice(start, start + pageSize);
  return {
    data,
    pagination: {
      page: safePage,
      pageSize,
      total,
      totalPages
    }
  };
}

function requireLegacyAuth(req, res, next) {
  const token = auth.extractToken(req);
  const session = auth.getSession(token);

  if (!session) {
    return res.status(401).json({ error: 'unauthorized' });
  }

  req.user = session;
  req.token = token;
  return next();
}

function authenticateV1(req, res, next) {
  const apiKey = req.headers['x-api-key'];
  if (apiKey) {
    const key = db.verifyApiKey(String(apiKey));
    if (!key || !key.enabled) {
      return sendError(req, res, 401, 'UNAUTHORIZED', 'Invalid API key');
    }

    req.authType = 'api_key';
    req.user = {
      id: `key-${key.id}`,
      username: key.name,
      role: key.role,
      source: 'api_key'
    };
    return next();
  }

  const token = auth.extractToken(req);
  const session = auth.getSession(token);
  if (!session) {
    return sendError(req, res, 401, 'UNAUTHORIZED', 'Authentication required');
  }

  req.authType = 'jwt';
  req.token = token;
  req.user = {
    id: String(session.userId),
    username: session.username,
    role: session.role === 'admin' ? 'administrator' : session.role,
    source: 'jwt'
  };
  return next();
}

function rateLimitV1(req, res, next) {
  const isApiKey = req.authType === 'api_key';
  const limit = isApiKey ? 5000 : 1000;
  const now = Date.now();
  const windowMs = 60 * 60 * 1000;
  const bucket = Math.floor(now / windowMs);
  const key = `${isApiKey ? 'k' : 'u'}:${req.user.id}:${bucket}`;

  let entry = rateCounters.get(key);
  if (!entry) {
    entry = { count: 0, resetAt: (bucket + 1) * windowMs };
    rateCounters.set(key, entry);
  }

  entry.count += 1;

  const remaining = Math.max(0, limit - entry.count);
  res.setHeader('X-RateLimit-Limit', String(limit));
  res.setHeader('X-RateLimit-Remaining', String(remaining));
  res.setHeader('X-RateLimit-Reset', String(Math.floor(entry.resetAt / 1000)));

  if (entry.count > limit) {
    return sendError(req, res, 429, 'RATE_LIMIT_EXCEEDED', 'Too many requests');
  }

  if (rateCounters.size > 20000) {
    const currentBucket = Math.floor(now / windowMs);
    for (const [k] of rateCounters.entries()) {
      const parts = k.split(':');
      const b = Number(parts[2]);
      if (b < currentBucket) {
        rateCounters.delete(k);
      }
    }
  }

  return next();
}

app.use(express.json());
app.use((req, _res, next) => {
  req.requestId = crypto.randomBytes(8).toString('hex');
  next();
});
// Static frontend is now served by Vite dev server (client/)

app.get('/health', (_req, res) => {
  res.json({ ok: true, service: 'rayavriti-netmonitor', timestamp: new Date().toISOString() });
});

app.post('/api/auth/login', (req, res) => {
  const { username, password } = req.body;
  if (!username || !password) {
    return res.status(400).json({ error: 'username and password are required' });
  }

  const session = auth.login(username, password);
  if (!session) {
    return res.status(401).json({ error: 'invalid credentials' });
  }

  return res.json({
    data: {
      token: session.token,
      user: {
        id: session.user.id,
        username: session.user.username,
        role: session.user.role
      }
    }
  });
});

app.get('/api/auth/me', requireLegacyAuth, (req, res) => {
  res.json({ data: req.user });
});

app.post('/api/auth/logout', requireLegacyAuth, (req, res) => {
  auth.logout(req.token, req.body?.refreshToken || null);
  res.json({ success: true });
});

app.get('/api/devices', requireLegacyAuth, (_req, res) => {
  res.json({ data: db.getDevices().map((device) => sanitizeDevice(device)) });
});

app.post('/api/devices', requireLegacyAuth, (req, res) => {
  const { name, host, protocol } = req.body;
  if (!name || !host || !protocol) {
    return res.status(400).json({ error: 'name, host, protocol are required' });
  }

  const result = db.addDevice(req.body);
  startScheduler(io);

  return res.status(201).json({
    data: sanitizeDevice(db.getDevice(result.lastInsertRowid))
  });
});

app.put('/api/devices/:id', requireLegacyAuth, (req, res) => {
  const id = Number(req.params.id);
  const existing = db.getDevice(id);
  if (!existing) {
    return res.status(404).json({ error: 'device not found' });
  }

  db.updateDevice(id, { ...existing, ...req.body });
  startScheduler(io);

  return res.json({ data: sanitizeDevice(db.getDevice(id)) });
});

app.delete('/api/devices/:id', requireLegacyAuth, (req, res) => {
  const id = Number(req.params.id);
  db.deleteDevice(id);
  startScheduler(io);
  res.status(204).send();
});

app.get('/api/metrics/latest', requireLegacyAuth, (_req, res) => {
  res.json({ data: db.getLatestMetrics() });
});

app.get('/api/metrics/device/:id', requireLegacyAuth, (req, res) => {
  const id = Number(req.params.id);
  const limit = Number(req.query.limit || 100);
  res.json({ data: db.getDeviceMetrics(id, limit) });
});

app.get('/api/alerts', requireLegacyAuth, (req, res) => {
  const status = req.query.status || 'active';
  const limit = Number(req.query.limit || 200);
  res.json({ data: db.getAlerts({ status, limit }) });
});

app.post('/api/alerts/:id/acknowledge', requireLegacyAuth, (req, res) => {
  db.acknowledgeAlert(Number(req.params.id), req.body?.comment || null);
  res.json({ success: true });
});

app.post('/api/alerts/:id/resolve', requireLegacyAuth, (req, res) => {
  db.resolveAlert(Number(req.params.id));
  res.json({ success: true });
});

app.get('/api/stats', requireLegacyAuth, (_req, res) => {
  res.json({ data: db.getStats() });
});

app.get('/api/alerts/counts', requireLegacyAuth, (_req, res) => {
  res.json({ data: db.getAlertCounts() });
});

app.get('/api/reports/summary', requireLegacyAuth, (req, res) => {
  const range = parseReportRange(req);
  if (!range) {
    return res.status(400).json({ error: 'invalid date range' });
  }

  const rows = db.getMetricsForReport({ ...range, deviceId: req.query.deviceId });
  const totalSamples = rows.length;
  const downSamples = rows.filter((r) => r.status === 'down').length;
  const warningSamples = rows.filter((r) => r.status === 'warning' || r.status === 'degraded').length;
  const avgResponse = rows.length
    ? Number((rows.reduce((sum, r) => sum + Number(r.response_time || 0), 0) / rows.length).toFixed(2))
    : 0;

  return res.json({
    data: {
      from: range.from,
      to: range.to,
      totalSamples,
      downSamples,
      warningSamples,
      availabilityPercent: totalSamples ? Number((((totalSamples - downSamples) / totalSamples) * 100).toFixed(2)) : 0,
      averageResponseMs: avgResponse
    }
  });
});

app.get('/api/reports/metrics.csv', requireLegacyAuth, (req, res) => {
  const range = parseReportRange(req);
  if (!range) {
    return res.status(400).json({ error: 'invalid date range' });
  }

  const rows = db.getMetricsForReport({ ...range, deviceId: req.query.deviceId });
  const csv = toCsv(rows);

  res.setHeader('Content-Type', 'text/csv; charset=utf-8');
  res.setHeader('Content-Disposition', 'attachment; filename="metrics-report.csv"');
  return res.send(csv);
});

app.get('/api/reports/timeseries', requireLegacyAuth, (req, res) => {
  const range = parseReportRange(req);
  if (!range) {
    return res.status(400).json({ error: 'invalid date range' });
  }

  const bucketMinutes = Number(req.query.bucketMinutes || 30);
  const rows = db.getReportTimeseries({
    ...range,
    deviceId: req.query.deviceId,
    bucketMinutes
  });

  const data = rows.map((row) => {
    const sampleCount = Number(row.sample_count || 0);
    const downCount = Number(row.down_count || 0);
    const availability = sampleCount > 0 ? Number((((sampleCount - downCount) / sampleCount) * 100).toFixed(2)) : 0;
    return {
      timestamp: row.bucket_time,
      sampleCount,
      downCount,
      warningCount: Number(row.warn_count || 0),
      avgResponseMs: Number(Number(row.avg_response || 0).toFixed(2)),
      availabilityPercent: availability
    };
  });

  return res.json({ data });
});

const v1 = express.Router();
v1.use(authenticateV1, rateLimitV1);

v1.post('/auth/logout', (req, res) => {
  auth.logout(req.token || null, req.body?.refreshToken || null);
  return sendOk(req, res, { loggedOut: true });
});

v1.get('/devices', (req, res) => {
  const { page, pageSize } = parsePagination(req.query);
  const statusFilter = req.query['filter[status]'];
  const sort = req.query.sort || '-created_at';

  const latestByDevice = new Map(db.getLatestMetrics().map((m) => [m.device_id, m]));
  const sensorsByDevice = new Map();
  for (const sensor of db.listSensors()) {
    const key = sensor.device_id;
    sensorsByDevice.set(key, (sensorsByDevice.get(key) || 0) + 1);
  }

  let rows = db.getDevices().map((d) => {
    const latest = latestByDevice.get(d.id);
    const state = latest?.status === 'down' ? 'down' : (latest?.status || 'active');
    return {
      id: String(d.id),
      name: d.name,
      type: d.type,
      ipAddress: d.host,
      status: d.enabled ? state : 'inactive',
      sensorCount: sensorsByDevice.get(d.id) || 0,
      created_at: d.created_at
    };
  });

  if (statusFilter) {
    rows = rows.filter((r) => String(r.status).toLowerCase() === String(statusFilter).toLowerCase());
  }

  rows = applySort(rows, sort, {
    created_at: 'created_at',
    name: 'name',
    status: 'status',
    type: 'type'
  });

  const paged = paginate(rows, page, pageSize);
  return sendOk(req, res, paged.data, { pagination: paged.pagination });
});

v1.get('/devices/:id', (req, res) => {
  const device = db.getDevice(Number(req.params.id));
  if (!device) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  const latest = db.getLatestMetrics().find((m) => m.device_id === device.id);
  const sensors = db.getSensorsByDevice(device.id);

  return sendOk(req, res, {
    id: String(device.id),
    name: device.name,
    type: device.type,
    ipAddress: device.host,
    status: latest?.status || 'unknown',
    sensorCount: sensors.length,
    protocol: device.protocol,
    port: device.port,
    snmpVersion: device.snmp_version || null,
    interval: device.interval_seconds,
    created_at: device.created_at
  });
});

v1.post('/devices', (req, res) => {
  const payload = req.body || {};
  const host = payload.ipAddress || payload.host;
  if (!payload.name || !host) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'name and ipAddress are required');
  }

  const created = db.addDevice({
    name: payload.name,
    type: payload.type || 'server',
    host,
    port: Number(payload.port || 0),
    protocol: payload.protocol || 'ping',
    snmpCommunity: payload.snmpCommunity,
    snmpVersion: payload.snmpVersion,
    interval: Number(payload.interval || 60)
  });

  startScheduler(io);
  const device = db.getDevice(created.lastInsertRowid);

  return sendOk(req, res, {
    id: String(device.id),
    name: device.name,
    type: device.type,
    ipAddress: device.host,
    status: 'active',
    sensorCount: db.getSensorsByDevice(device.id).length
  }, {}, 201);
});

v1.put('/devices/:id', (req, res) => {
  const id = Number(req.params.id);
  const existing = db.getDevice(id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  const payload = req.body || {};
  db.updateDevice(id, {
    ...existing,
    name: payload.name ?? existing.name,
    type: payload.type ?? existing.type,
    host: payload.ipAddress ?? payload.host ?? existing.host,
    port: payload.port ?? existing.port,
    protocol: payload.protocol ?? existing.protocol,
    snmpCommunity: payload.snmpCommunity ?? existing.snmp_community,
    snmpVersion: payload.snmpVersion ?? existing.snmp_version,
    interval: payload.interval ?? payload.interval_seconds ?? existing.interval_seconds
  });

  startScheduler(io);
  const device = db.getDevice(id);

  return sendOk(req, res, {
    id: String(device.id),
    name: device.name,
    type: device.type,
    ipAddress: device.host,
    protocol: device.protocol,
    interval: device.interval_seconds,
    port: device.port,
    snmpVersion: device.snmp_version || null
  });
});

v1.delete('/devices/:id', (req, res) => {
  const id = Number(req.params.id);
  const existing = db.getDevice(id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  db.deleteDevice(id);
  startScheduler(io);
  return sendOk(req, res, { deleted: true });
});

v1.get('/devices/:id/metrics', (req, res) => {
  const id = Number(req.params.id);
  if (!db.getDevice(id)) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  const limit = Number(req.query.limit || 200);
  const rows = db.getDeviceMetrics(id, limit);
  return sendOk(req, res, rows);
});

v1.get('/sensors', (req, res) => {
  const rows = db.listSensors({ deviceId: req.query.deviceId });
  return sendOk(req, res, rows.map((s) => ({
    id: String(s.id),
    deviceId: String(s.device_id),
    deviceName: s.device_name,
    name: s.name,
    type: s.type,
    interval: s.interval_seconds,
    config: (() => {
      try {
        return JSON.parse(s.config_json || '{}');
      } catch (_e) {
        return {};
      }
    })(),
    enabled: Boolean(s.enabled),
    created_at: s.created_at
  })));
});

v1.get('/sensors/:id', (req, res) => {
  const sensor = db.getSensor(req.params.id);
  if (!sensor) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Sensor not found');
  }

  return sendOk(req, res, sensor);
});

v1.post('/sensors', (req, res) => {
  const payload = req.body || {};
  if (!payload.deviceId || !payload.name || !payload.type) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'deviceId, name and type are required');
  }

  const device = db.getDevice(Number(payload.deviceId));
  if (!device) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  const result = db.addSensor(payload);
  const sensor = db.getSensor(result.lastInsertRowid);
  return sendOk(req, res, sensor, {}, 201);
});

v1.put('/sensors/:id', (req, res) => {
  const existing = db.getSensor(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Sensor not found');
  }

  const payload = req.body || {};
  db.updateSensor(req.params.id, {
    ...existing,
    name: payload.name ?? existing.name,
    type: payload.type ?? existing.type,
    interval: payload.interval ?? payload.interval_seconds ?? existing.interval_seconds,
    config: payload.config ?? (() => {
      try {
        return JSON.parse(existing.config_json || '{}');
      } catch (_e) {
        return {};
      }
    })(),
    enabled: typeof payload.enabled === 'boolean' ? payload.enabled : Boolean(existing.enabled)
  });

  const updated = db.getSensor(req.params.id);
  return sendOk(req, res, updated);
});

v1.delete('/sensors/:id', (req, res) => {
  const existing = db.getSensor(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Sensor not found');
  }

  db.deleteSensor(req.params.id);
  return sendOk(req, res, { deleted: true });
});

v1.get('/metrics/query', (req, res) => {
  const { deviceId, from, to, aggregation = 'avg', interval = '5m' } = req.query;
  if (!deviceId || !from || !to) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'deviceId, from and to are required');
  }

  const fromDate = new Date(from);
  const toDate = new Date(to);
  if (Number.isNaN(fromDate.getTime()) || Number.isNaN(toDate.getTime())) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'Invalid from/to datetime');
  }

  const rows = db.queryMetrics({
    deviceId: Number(deviceId),
    from: toSqlDate(fromDate),
    to: toSqlDate(toDate)
  });

  const bucketMap = {
    '1m': 60,
    '5m': 300,
    '1h': 3600,
    '1d': 86400
  };
  const bucketSec = bucketMap[interval] || 300;

  const groups = new Map();
  for (const row of rows) {
    const sensorName = row.sensor_name || 'Default Sensor';
    const ts = Math.floor(new Date(row.timestamp).getTime() / 1000);
    const bucket = Math.floor(ts / bucketSec) * bucketSec;
    const key = `${sensorName}:${bucket}`;
    const val = Number(row.value ?? row.response_time ?? 0);

    if (!groups.has(key)) {
      groups.set(key, {
        sensorName,
        bucket,
        values: []
      });
    }

    groups.get(key).values.push(val);
  }

  const bySensor = new Map();
  for (const g of groups.values()) {
    let value = 0;
    if (aggregation === 'min') {
      value = Math.min(...g.values);
    } else if (aggregation === 'max') {
      value = Math.max(...g.values);
    } else {
      value = g.values.reduce((sum, x) => sum + x, 0) / g.values.length;
    }

    if (!bySensor.has(g.sensorName)) {
      bySensor.set(g.sensorName, []);
    }

    bySensor.get(g.sensorName).push({
      timestamp: new Date(g.bucket * 1000).toISOString(),
      value: Number(value.toFixed(3))
    });
  }

  const series = Array.from(bySensor.entries()).map(([sensorName, dataPoints]) => ({
    sensorName,
    dataPoints: dataPoints.sort((a, b) => a.timestamp.localeCompare(b.timestamp))
  }));

  return sendOk(req, res, { series });
});

v1.get('/alerts', (req, res) => {
  const { page, pageSize } = parsePagination(req.query);
  const statusFilter = req.query.status || req.query['filter[status]'] || 'all';
  const sort = req.query.sort || '-created_at';

  let rows = db.getAlerts({ status: statusFilter === 'triggered' ? 'active' : statusFilter, limit: 5000 });

  rows = rows.map((a) => ({
    id: String(a.id),
    severity: a.severity,
    status: a.status === 'active' ? 'triggered' : a.status,
    message: a.message,
    deviceName: a.device_name,
    triggeredAt: new Date(a.created_at).toISOString(),
    comment: a.comment || null,
    created_at: a.created_at
  }));

  rows = applySort(rows, sort, {
    created_at: 'created_at',
    severity: 'severity',
    status: 'status'
  });

  const paged = paginate(rows, page, pageSize);
  return sendOk(req, res, paged.data, { pagination: paged.pagination });
});

v1.get('/alerts/:id', (req, res) => {
  const alert = db.getAlertById(req.params.id);
  if (!alert) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Alert not found');
  }

  return sendOk(req, res, {
    id: String(alert.id),
    severity: alert.severity,
    status: alert.status === 'active' ? 'triggered' : alert.status,
    message: alert.message,
    deviceName: alert.device_name,
    triggeredAt: new Date(alert.created_at).toISOString(),
    comment: alert.comment || null
  });
});

v1.post('/alerts', (req, res) => {
  const payload = req.body || {};
  if (!payload.deviceId || !payload.message || !payload.severity) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'deviceId, message and severity are required');
  }

  const device = db.getDevice(Number(payload.deviceId));
  if (!device) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Device not found');
  }

  const status = payload.status === 'triggered' ? 'active' : (payload.status || 'active');
  const created = db.createAlert(payload.deviceId, payload.severity, payload.message, status, payload.comment || null);
  const alert = db.getAlertById(created.lastInsertRowid);
  return sendOk(req, res, alert, {}, 201);
});

v1.put('/alerts/:id', (req, res) => {
  const existing = db.getAlertById(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Alert not found');
  }

  const payload = req.body || {};
  db.updateAlert(req.params.id, {
    severity: payload.severity ?? existing.severity,
    message: payload.message ?? existing.message,
    status: payload.status === 'triggered' ? 'active' : (payload.status ?? existing.status),
    comment: payload.comment ?? existing.comment
  });

  return sendOk(req, res, db.getAlertById(req.params.id));
});

v1.delete('/alerts/:id', (req, res) => {
  const existing = db.getAlertById(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Alert not found');
  }

  db.deleteAlert(req.params.id);
  return sendOk(req, res, { deleted: true });
});

v1.post('/alerts/:id/acknowledge', (req, res) => {
  const existing = db.getAlertById(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Alert not found');
  }

  db.acknowledgeAlert(req.params.id, req.body?.comment || null);
  return sendOk(req, res, { acknowledged: true });
});

v1.post('/alerts/:id/resolve', (req, res) => {
  const existing = db.getAlertById(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Alert not found');
  }

  db.resolveAlert(req.params.id);
  return sendOk(req, res, { resolved: true });
});

v1.get('/dashboards', (req, res) => {
  const dashboards = db.listDashboards().map((d) => ({
    id: String(d.id),
    name: d.name,
    widgets: (() => {
      try {
        return JSON.parse(d.widgets_json || '[]');
      } catch (_e) {
        return [];
      }
    })(),
    createdAt: d.created_at,
    updatedAt: d.updated_at
  }));

  return sendOk(req, res, dashboards);
});

v1.get('/dashboards/:id', (req, res) => {
  const d = db.getDashboard(req.params.id);
  if (!d) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Dashboard not found');
  }

  return sendOk(req, res, {
    id: String(d.id),
    name: d.name,
    widgets: (() => {
      try {
        return JSON.parse(d.widgets_json || '[]');
      } catch (_e) {
        return [];
      }
    })(),
    createdAt: d.created_at,
    updatedAt: d.updated_at
  });
});

v1.post('/dashboards', (req, res) => {
  const payload = req.body || {};
  if (!payload.name) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'name is required');
  }

  const created = db.createDashboard({
    name: payload.name,
    widgets: payload.widgets || [],
    createdBy: Number(req.user.id) || null
  });

  return sendOk(req, res, { id: String(created.lastInsertRowid) }, {}, 201);
});

v1.put('/dashboards/:id', (req, res) => {
  const existing = db.getDashboard(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Dashboard not found');
  }

  const payload = req.body || {};
  db.updateDashboard(req.params.id, {
    name: payload.name ?? existing.name,
    widgets: payload.widgets ?? (() => {
      try {
        return JSON.parse(existing.widgets_json || '[]');
      } catch (_e) {
        return [];
      }
    })()
  });

  return sendOk(req, res, { updated: true });
});

v1.delete('/dashboards/:id', (req, res) => {
  const existing = db.getDashboard(req.params.id);
  if (!existing) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Dashboard not found');
  }

  db.deleteDashboard(req.params.id);
  return sendOk(req, res, { deleted: true });
});

v1.get('/reports', (req, res) => {
  return sendOk(req, res, [
    { id: 'availability', name: 'Availability Report' },
    { id: 'performance', name: 'Performance Report' },
    { id: 'sla', name: 'SLA Report' }
  ]);
});

// ── Flow Analysis Routes ──────────────────────────────────────

v1.get('/flows', (req, res) => {
  const { from, to, srcIp, dstIp, protocol } = req.query;
  const limit = Number(req.query.limit || 200);
  const rows = db.getFlowRecords({ from, to, srcIp, dstIp, protocol, limit });
  return sendOk(req, res, rows);
});

v1.get('/flows/top-talkers', (req, res) => {
  const { from, to, direction } = req.query;
  const limit = Number(req.query.limit || 10);
  const data = flowAnalyzer.getTopTalkersWithPercent({ from, to, limit, direction });
  return sendOk(req, res, data);
});

v1.get('/flows/protocols', (req, res) => {
  const { from, to } = req.query;
  const data = flowAnalyzer.getProtocolBreakdown({ from, to });
  return sendOk(req, res, data);
});

v1.get('/flows/timeseries', (req, res) => {
  const { from, to } = req.query;
  const bucketMinutes = Number(req.query.bucketMinutes || 5);
  const data = db.getFlowTimeseries({ from, to, bucketMinutes });
  return sendOk(req, res, data.map((r) => ({
    timestamp: r.bucket_time,
    totalBytes: Number(r.total_bytes || 0),
    totalPackets: Number(r.total_packets || 0),
    flowCount: Number(r.flow_count || 0)
  })));
});

v1.get('/flows/stats', (req, res) => {
  const data = flowAnalyzer.getFlowSummary();
  return sendOk(req, res, data);
});

// ── Packet Capture Routes ─────────────────────────────────────

v1.get('/capture/interfaces', (req, res) => {
  const interfaces = listInterfaces();
  return sendOk(req, res, interfaces);
});

v1.post('/capture/start', (req, res) => {
  const { interface: iface, filter } = req.body || {};
  if (!iface) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'interface is required');
  }
  try {
    const sessionId = startCapture(io, iface, filter || null);
    const session = db.getCaptureSession(sessionId);
    return sendOk(req, res, session, {}, 201);
  } catch (err) {
    return sendError(req, res, 500, 'CAPTURE_ERROR', err.message);
  }
});

v1.post('/capture/:id/stop', (req, res) => {
  const id = Number(req.params.id);
  const stopped = stopCapture(io, id);
  if (!stopped) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Capture session not found or already stopped');
  }
  const session = db.getCaptureSession(id);
  return sendOk(req, res, session);
});

v1.get('/capture/:id', (req, res) => {
  const id = Number(req.params.id);
  const session = db.getCaptureSession(id);
  if (!session) {
    return sendError(req, res, 404, 'RESOURCE_NOT_FOUND', 'Capture session not found');
  }
  const liveStats = getSessionStats(id);
  return sendOk(req, res, {
    ...session,
    ...(liveStats ? { packet_count: liveStats.packetCount, bytes_captured: liveStats.bytesCaptured } : {})
  });
});

v1.get('/capture/:id/packets', (req, res) => {
  const id = Number(req.params.id);
  const limit = Number(req.query.limit || 200);
  const offset = Number(req.query.offset || 0);
  const packets = getSessionPackets(id, { limit, offset });
  return sendOk(req, res, packets);
});

v1.get('/capture/sessions', (req, res) => {
  const limit = Number(req.query.limit || 50);
  const sessions = db.getCaptureSessions(limit);
  return sendOk(req, res, sessions);
});

app.post('/api/v1/auth/login', (req, res) => {
  const { username, password } = req.body || {};
  if (!username || !password) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'username and password are required');
  }

  const session = auth.login(username, password);
  if (!session) {
    return sendError(req, res, 401, 'UNAUTHORIZED', 'Invalid credentials');
  }

  return sendOk(req, res, {
    accessToken: session.accessToken,
    refreshToken: session.refreshToken,
    expiresIn: session.expiresIn,
    user: session.user
  });
});

app.post('/api/v1/auth/refresh', (req, res) => {
  const { refreshToken } = req.body || {};
  if (!refreshToken) {
    return sendError(req, res, 400, 'VALIDATION_ERROR', 'refreshToken is required');
  }

  const refreshed = auth.refresh(refreshToken);
  if (!refreshed) {
    return sendError(req, res, 401, 'UNAUTHORIZED', 'Invalid refresh token');
  }

  return sendOk(req, res, refreshed);
});

app.post('/api/v1/auth/2fa/verify', authenticateV1, rateLimitV1, (req, res) => {
  return sendOk(req, res, { verified: true });
});

app.use('/api/v1', v1);

io.use((socket, next) => {
  const token = socket.handshake.auth?.token || socket.handshake.headers['x-session-token'];
  const session = auth.getSession(token);
  if (!session) {
    return next(new Error('unauthorized'));
  }

  socket.data.user = session;
  return next();
});

io.on('connection', (socket) => {
  socket.emit('bootstrap', {
    stats: db.getStats(),
    latestMetrics: db.getLatestMetrics(),
    alerts: db.getActiveAlerts(),
    user: socket.data.user
  });

  socket.on('disconnect', () => {
    // No per-client resources to dispose yet.
  });
});

server.listen(PORT, () => {
  console.log(`Rayavriti NetMonitor running on http://localhost:${PORT}`);
  startScheduler(io);
  startNetflowCollector(io, Number(process.env.NETFLOW_PORT || 2055));
});

process.on('SIGINT', () => {
  clearJobs();
  stopNetflowCollector();
  stopAllCaptures(io);
  process.exit(0);
});
