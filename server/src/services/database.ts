const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const Database = require('better-sqlite3');

const dbDir = path.resolve(process.cwd(), 'data');
fs.mkdirSync(dbDir, { recursive: true });
const dbPath = path.join(dbDir, 'netmonitor.db');
const db = new Database(dbPath);

db.pragma('journal_mode = WAL');

function hashApiKey(key) {
  return crypto.createHash('sha256').update(key).digest('hex');
}

function ensureColumn(table, column, ddl) {
  const cols = db.prepare(`PRAGMA table_info(${table})`).all();
  if (!cols.some((c) => c.name === column)) {
    db.exec(`ALTER TABLE ${table} ADD COLUMN ${ddl}`);
  }
}

db.exec(`
  CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'admin',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT DEFAULT 'server',
    host TEXT NOT NULL,
    port INTEGER DEFAULT 0,
    protocol TEXT DEFAULT 'ping',
    snmp_community TEXT,
    snmp_version TEXT,
    interval_seconds INTEGER DEFAULT 60,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE IF NOT EXISTS sensors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    interval_seconds INTEGER DEFAULT 60,
    config_json TEXT,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
  );

  CREATE TABLE IF NOT EXISTS metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    sensor_id INTEGER,
    status TEXT NOT NULL,
    response_time REAL,
    value REAL,
    message TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
  );

  CREATE TABLE IF NOT EXISTS alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at DATETIME,
    resolved_at DATETIME,
    comment TEXT,
    FOREIGN KEY (device_id) REFERENCES devices(id)
  );

  CREATE TABLE IF NOT EXISTS dashboards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    widgets_json TEXT NOT NULL,
    created_by INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
  );

  CREATE TABLE IF NOT EXISTS api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    role TEXT DEFAULT 'administrator',
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE IF NOT EXISTS flow_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collector_type TEXT NOT NULL,
    src_ip TEXT NOT NULL,
    dst_ip TEXT NOT NULL,
    src_port INTEGER,
    dst_port INTEGER,
    protocol INTEGER,
    protocol_name TEXT,
    bytes INTEGER DEFAULT 0,
    packets INTEGER DEFAULT 0,
    flow_start DATETIME,
    flow_end DATETIME,
    input_interface INTEGER,
    output_interface INTEGER,
    tcp_flags INTEGER,
    tos INTEGER,
    src_as INTEGER,
    dst_as INTEGER,
    exporter_ip TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE IF NOT EXISTS capture_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    interface_name TEXT NOT NULL,
    filter TEXT,
    status TEXT DEFAULT 'running',
    packet_count INTEGER DEFAULT 0,
    bytes_captured INTEGER DEFAULT 0,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    stopped_at DATETIME,
    error_message TEXT
  );

  CREATE TABLE IF NOT EXISTS port_scan_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    port INTEGER NOT NULL,
    status TEXT NOT NULL,
    service_guess TEXT,
    response_time REAL,
    first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_changed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id),
    UNIQUE(device_id, port)
  );

  CREATE INDEX IF NOT EXISTS idx_metrics_device_time ON metrics(device_id, timestamp);
  CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
  CREATE INDEX IF NOT EXISTS idx_sensors_device ON sensors(device_id);
  CREATE INDEX IF NOT EXISTS idx_flow_src_ip ON flow_records(src_ip, timestamp);
  CREATE INDEX IF NOT EXISTS idx_flow_dst_ip ON flow_records(dst_ip, timestamp);
  CREATE INDEX IF NOT EXISTS idx_flow_time ON flow_records(timestamp);
  CREATE INDEX IF NOT EXISTS idx_port_scan_device ON port_scan_results(device_id, status);
`);

ensureColumn('metrics', 'sensor_id', 'sensor_id INTEGER');
ensureColumn('alerts', 'resolved_at', 'resolved_at DATETIME');
ensureColumn('alerts', 'comment', 'comment TEXT');
ensureColumn('devices', 'snmp_community', 'snmp_community TEXT');
ensureColumn('devices', 'snmp_version', 'snmp_version TEXT');
db.exec('CREATE INDEX IF NOT EXISTS idx_metrics_sensor_time ON metrics(sensor_id, timestamp)');

const defaultDevices = [
  { name: 'Gateway', type: 'network', host: '1.1.1.1', protocol: 'ping', interval: 30 },
  { name: 'Google DNS', type: 'network', host: '8.8.8.8', protocol: 'ping', interval: 30 },
  { name: 'Rayavriti Website', type: 'service', host: 'https://example.com', protocol: 'http', interval: 60 },
  { name: 'Localhost API Port', type: 'service', host: '127.0.0.1', port: 3000, protocol: 'port', interval: 30 },
  { name: 'Local System', type: 'server', host: 'localhost', protocol: 'system', interval: 20 }
];

const defaultUsers = [
  {
    username: 'admin',
    // demo password: admin123
    passwordHash: '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9'
  }
];

const defaultApiKeys = [
  {
    name: 'Default Integration Key',
    key: 'sk_live_demo123'
  }
];

function seedUsers() {
  const upsert = db.prepare(`
    INSERT INTO users (username, password_hash, role)
    VALUES (@username, @passwordHash, 'admin')
    ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash
  `);

  const tx = db.transaction((users) => {
    for (const user of users) {
      upsert.run(user);
    }
  });

  tx(defaultUsers);
}

function seedDefaults() {
  const count = db.prepare('SELECT COUNT(*) AS count FROM devices').get().count;
  if (count > 0) {
    return;
  }

  const insert = db.prepare(`
    INSERT INTO devices (name, type, host, port, protocol, interval_seconds, enabled)
    VALUES (@name, @type, @host, @port, @protocol, @interval, 1)
  `);

  const tx = db.transaction((devices) => {
    for (const device of devices) {
      insert.run({
        name: device.name,
        type: device.type || 'server',
        host: device.host,
        port: device.port || 0,
        protocol: device.protocol || 'ping',
        interval: device.interval || 60
      });
    }
  });

  tx(defaultDevices);
}

function ensureDefaultSensorsForAll() {
  const devices = db.prepare('SELECT * FROM devices WHERE enabled = 1').all();
  const hasAny = db.prepare('SELECT COUNT(*) AS c FROM sensors').get().c;
  if (hasAny > 0 && devices.length > 0) {
    for (const d of devices) {
      const one = db.prepare('SELECT id FROM sensors WHERE device_id = ? LIMIT 1').get(d.id);
      if (!one) {
        db.prepare(`
          INSERT INTO sensors (device_id, name, type, interval_seconds, config_json, enabled)
          VALUES (?, ?, ?, ?, ?, 1)
        `).run(d.id, `${d.name} ${String(d.protocol).toUpperCase()} Sensor`, d.protocol, d.interval_seconds || 60, '{}');
      }
    }
    return;
  }

  const insert = db.prepare(`
    INSERT INTO sensors (device_id, name, type, interval_seconds, config_json, enabled)
    VALUES (?, ?, ?, ?, ?, 1)
  `);
  const tx = db.transaction((all) => {
    for (const d of all) {
      insert.run(
        d.id,
        `${d.name} ${String(d.protocol).toUpperCase()} Sensor`,
        d.protocol,
        d.interval_seconds || 60,
        '{}'
      );
    }
  });

  tx(devices);
}

function seedApiKeys() {
  const insert = db.prepare(`
    INSERT INTO api_keys (name, key_hash, role, enabled)
    VALUES (?, ?, 'administrator', 1)
    ON CONFLICT(key_hash) DO NOTHING
  `);

  for (const entry of defaultApiKeys) {
    insert.run(entry.name, hashApiKey(entry.key));
  }
}

seedUsers();
seedDefaults();
ensureDefaultSensorsForAll();
seedApiKeys();

module.exports = {
  getDevices: () => db.prepare('SELECT * FROM devices WHERE enabled = 1 ORDER BY id DESC').all(),

  getDevice: (id) => db.prepare('SELECT * FROM devices WHERE id = ?').get(id),

  addDevice: (device) => {
    const snmpCommunity = device.protocol === 'snmp'
      ? (device.snmpCommunity || device.snmp_community || 'public')
      : null;
    const snmpVersion = device.protocol === 'snmp'
      ? (device.snmpVersion || device.snmp_version || '2c')
      : null;
    const stmt = db.prepare(`
      INSERT INTO devices (name, type, host, port, protocol, snmp_community, snmp_version, interval_seconds, enabled)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
    `);

    const result = stmt.run(
      device.name,
      device.type || 'server',
      device.host,
      Number(device.port || 0),
      device.protocol || 'ping',
      snmpCommunity,
      snmpVersion,
      Number(device.interval || device.interval_seconds || 60)
    );

    db.prepare(`
      INSERT INTO sensors (device_id, name, type, interval_seconds, config_json, enabled)
      VALUES (?, ?, ?, ?, ?, 1)
    `).run(
      result.lastInsertRowid,
      `${device.name} ${String(device.protocol || 'ping').toUpperCase()} Sensor`,
      device.protocol || 'ping',
      Number(device.interval || device.interval_seconds || 60),
      '{}'
    );

    return result;
  },

  updateDevice: (id, device) => {
    const snmpCommunity = device.protocol === 'snmp'
      ? (device.snmpCommunity || device.snmp_community || 'public')
      : null;
    const snmpVersion = device.protocol === 'snmp'
      ? (device.snmpVersion || device.snmp_version || '2c')
      : null;
    const stmt = db.prepare(`
      UPDATE devices
      SET name = ?, type = ?, host = ?, port = ?, protocol = ?, snmp_community = ?, snmp_version = ?, interval_seconds = ?
      WHERE id = ?
    `);

    const result = stmt.run(
      device.name,
      device.type || 'server',
      device.host,
      Number(device.port || 0),
      device.protocol || 'ping',
      snmpCommunity,
      snmpVersion,
      Number(device.interval || device.interval_seconds || 60),
      id
    );

    db.prepare(`
      UPDATE sensors
      SET type = ?, interval_seconds = ?
      WHERE device_id = ?
    `).run(
      device.protocol || 'ping',
      Number(device.interval || device.interval_seconds || 60),
      id
    );

    return result;
  },

  deleteDevice: (id) => {
    db.prepare('DELETE FROM sensors WHERE device_id = ?').run(id);
    db.prepare('DELETE FROM metrics WHERE device_id = ?').run(id);
    db.prepare('DELETE FROM alerts WHERE device_id = ?').run(id);
    db.prepare('DELETE FROM port_scan_results WHERE device_id = ?').run(id);
    return db.prepare('DELETE FROM devices WHERE id = ?').run(id);
  },

  listSensors: ({ deviceId, enabled }: any = {}) => {
    const clauses = [];
    const params = [];

    if (deviceId) {
      clauses.push('s.device_id = ?');
      params.push(Number(deviceId));
    }

    if (typeof enabled !== 'undefined') {
      clauses.push('s.enabled = ?');
      params.push(enabled ? 1 : 0);
    }

    const whereSql = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return db.prepare(`
      SELECT s.*, d.name AS device_name
      FROM sensors s
      JOIN devices d ON d.id = s.device_id
      ${whereSql}
      ORDER BY s.id DESC
    `).all(...params);
  },

  getSensor: (id) => db.prepare(`
    SELECT s.*, d.name AS device_name
    FROM sensors s
    JOIN devices d ON d.id = s.device_id
    WHERE s.id = ?
  `).get(Number(id)),

  getSensorsByDevice: (deviceId) => db.prepare(`
    SELECT * FROM sensors WHERE device_id = ? ORDER BY id ASC
  `).all(Number(deviceId)),

  addSensor: (sensor) => db.prepare(`
    INSERT INTO sensors (device_id, name, type, interval_seconds, config_json, enabled)
    VALUES (?, ?, ?, ?, ?, ?)
  `).run(
    Number(sensor.deviceId),
    sensor.name,
    sensor.type,
    Number(sensor.interval || sensor.interval_seconds || 60),
    JSON.stringify(sensor.config || {}),
    sensor.enabled === false ? 0 : 1
  ),

  updateSensor: (id, sensor) => db.prepare(`
    UPDATE sensors
    SET name = ?, type = ?, interval_seconds = ?, config_json = ?, enabled = ?
    WHERE id = ?
  `).run(
    sensor.name,
    sensor.type,
    Number(sensor.interval || sensor.interval_seconds || 60),
    JSON.stringify(sensor.config || {}),
    sensor.enabled === false ? 0 : 1,
    Number(id)
  ),

  deleteSensor: (id) => {
    db.prepare('UPDATE metrics SET sensor_id = NULL WHERE sensor_id = ?').run(Number(id));
    return db.prepare('DELETE FROM sensors WHERE id = ?').run(Number(id));
  },

  getPrimarySensorForDevice: (deviceId) => db.prepare(`
    SELECT * FROM sensors
    WHERE device_id = ? AND enabled = 1
    ORDER BY id ASC
    LIMIT 1
  `).get(Number(deviceId)),

  recordMetric: (deviceId, status, responseTime, value, message, sensorId = null) => {
    const stmt = db.prepare(`
      INSERT INTO metrics (device_id, sensor_id, status, response_time, value, message)
      VALUES (?, ?, ?, ?, ?, ?)
    `);

    return stmt.run(deviceId, sensorId, status, responseTime, value, message);
  },

  getDeviceMetrics: (deviceId, limit = 100) => db
    .prepare(`
      SELECT m.*, s.name AS sensor_name
      FROM metrics m
      LEFT JOIN sensors s ON s.id = m.sensor_id
      WHERE m.device_id = ?
      ORDER BY m.timestamp DESC
      LIMIT ?
    `)
    .all(Number(deviceId), Number(limit)),

  queryMetrics: ({ deviceId, from, to }) => db.prepare(`
    SELECT m.*, s.name AS sensor_name
    FROM metrics m
    LEFT JOIN sensors s ON s.id = m.sensor_id
    WHERE m.device_id = ? AND m.timestamp BETWEEN ? AND ?
    ORDER BY m.timestamp ASC
    LIMIT 10000
  `).all(Number(deviceId), from, to),

  getRecentMetrics: ({ since, limit = 5000 }: any = {}) => {
    const clauses = [];
    const params = [];
    if (since) {
      clauses.push('m.timestamp >= ?');
      params.push(since);
    }
    const whereSql = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return db.prepare(`
      SELECT m.*, d.name AS device_name, d.type AS device_type, d.host, d.protocol
      FROM metrics m
      JOIN devices d ON d.id = m.device_id
      ${whereSql}
      ORDER BY m.timestamp DESC
      LIMIT ?
    `).all(...params, Number(limit));
  },

  getLatestMetrics: () => db.prepare(`
    SELECT m.*, d.name as device_name, d.type as device_type, d.host, d.protocol, s.name AS sensor_name
    FROM metrics m
    JOIN devices d ON m.device_id = d.id
    LEFT JOIN sensors s ON s.id = m.sensor_id
    WHERE m.id IN (SELECT MAX(id) FROM metrics GROUP BY device_id)
    ORDER BY m.timestamp DESC
  `).all(),

  getActiveAlerts: () => db.prepare(`
    SELECT a.*, d.name AS device_name
    FROM alerts a
    JOIN devices d ON a.device_id = d.id
    WHERE a.status = 'active'
    ORDER BY a.created_at DESC
  `).all(),

  getAlertById: (id) => db.prepare(`
    SELECT a.*, d.name AS device_name
    FROM alerts a
    JOIN devices d ON a.device_id = d.id
    WHERE a.id = ?
  `).get(Number(id)),

  getAlerts: ({ status = 'active', limit = 200 } = {}) => {
    const clauses = [];
    const params = [];
    if (status && status !== 'all') {
      clauses.push('a.status = ?');
      params.push(status);
    }

    const whereSql = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';

    return db.prepare(`
      SELECT a.*, d.name AS device_name
      FROM alerts a
      JOIN devices d ON a.device_id = d.id
      ${whereSql}
      ORDER BY a.created_at DESC
      LIMIT ?
    `).all(...params, Number(limit));
  },

  getAlertCounts: () => {
    const rows = db.prepare(`
      SELECT status, COUNT(*) AS total
      FROM alerts
      GROUP BY status
    `).all();

    const counts = {
      active: 0,
      acknowledged: 0,
      resolved: 0
    };

    for (const row of rows) {
      counts[row.status] = row.total;
    }

    return counts;
  },

  createAlert: (deviceId, severity, message, status = 'active', comment = null) => {
    const stmt = db.prepare('INSERT INTO alerts (device_id, severity, message, status, comment) VALUES (?, ?, ?, ?, ?)');
    return stmt.run(Number(deviceId), severity, message, status, comment);
  },

  findActiveAlert: (deviceId, message) => db.prepare(`
    SELECT *
    FROM alerts
    WHERE device_id = ? AND message = ? AND status = 'active'
    LIMIT 1
  `).get(Number(deviceId), message),

  updateAlert: (id, patch) => db.prepare(`
    UPDATE alerts
    SET severity = ?, message = ?, status = ?, comment = ?,
        acknowledged_at = CASE WHEN ? = 'acknowledged' THEN CURRENT_TIMESTAMP ELSE acknowledged_at END,
        resolved_at = CASE WHEN ? = 'resolved' THEN CURRENT_TIMESTAMP ELSE resolved_at END
    WHERE id = ?
  `).run(
    patch.severity,
    patch.message,
    patch.status,
    patch.comment || null,
    patch.status,
    patch.status,
    Number(id)
  ),

  deleteAlert: (id) => db.prepare('DELETE FROM alerts WHERE id = ?').run(Number(id)),

  acknowledgeAlert: (id, comment = null) => db
    .prepare('UPDATE alerts SET status = ?, comment = ?, acknowledged_at = CURRENT_TIMESTAMP WHERE id = ?')
    .run('acknowledged', comment, Number(id)),

  resolveAlert: (id) => db
    .prepare('UPDATE alerts SET status = ?, resolved_at = CURRENT_TIMESTAMP WHERE id = ?')
    .run('resolved', Number(id)),

  getStats: () => {
    const totalDevices = db.prepare('SELECT COUNT(*) AS total FROM devices WHERE enabled = 1').get().total;
    const activeAlerts = db.prepare('SELECT COUNT(*) AS total FROM alerts WHERE status = ?').get('active').total;

    const onlineDevices = db.prepare(`
      SELECT COUNT(*) AS total
      FROM (
        SELECT device_id, status
        FROM metrics
        WHERE id IN (SELECT MAX(id) FROM metrics GROUP BY device_id)
      ) latest
      WHERE latest.status = 'up' OR latest.status = 'ok'
    `).get().total;

    return {
      totalDevices,
      onlineDevices,
      activeAlerts,
      uptimePercent: totalDevices > 0 ? Number(((onlineDevices / totalDevices) * 100).toFixed(1)) : 0
    };
  },

  getUserByUsername: (username) => db
    .prepare('SELECT id, username, password_hash, role FROM users WHERE username = ?')
    .get(username),

  getMetricsForReport: ({ from, to, deviceId }) => {
    const clauses = ['m.timestamp BETWEEN ? AND ?'];
    const params = [from, to];

    if (deviceId) {
      clauses.push('m.device_id = ?');
      params.push(Number(deviceId));
    }

    return db.prepare(`
      SELECT
        m.id,
        m.device_id,
        d.name AS device_name,
        d.protocol,
        m.status,
        m.response_time,
        m.value,
        m.message,
        m.timestamp
      FROM metrics m
      JOIN devices d ON d.id = m.device_id
      WHERE ${clauses.join(' AND ')}
      ORDER BY m.timestamp DESC
      LIMIT 5000
    `).all(...params);
  },

  getReportTimeseries: ({ from, to, deviceId, bucketMinutes = 30 }) => {
    const clauses = ['m.timestamp BETWEEN ? AND ?'];
    const params = [from, to];
    if (deviceId) {
      clauses.push('m.device_id = ?');
      params.push(Number(deviceId));
    }

    const bucketSeconds = Math.max(1, Number(bucketMinutes)) * 60;

    return db.prepare(`
      SELECT
        datetime((cast(strftime('%s', m.timestamp) as integer) / ${bucketSeconds}) * ${bucketSeconds}, 'unixepoch') AS bucket_time,
        COUNT(*) AS sample_count,
        AVG(COALESCE(m.response_time, 0)) AS avg_response,
        SUM(CASE WHEN m.status = 'down' THEN 1 ELSE 0 END) AS down_count,
        SUM(CASE WHEN m.status IN ('warning', 'degraded') THEN 1 ELSE 0 END) AS warn_count
      FROM metrics m
      WHERE ${clauses.join(' AND ')}
      GROUP BY bucket_time
      ORDER BY bucket_time ASC
      LIMIT 2000
    `).all(...params);
  },

  listDashboards: () => db.prepare('SELECT * FROM dashboards ORDER BY updated_at DESC, id DESC').all(),

  getDashboard: (id) => db.prepare('SELECT * FROM dashboards WHERE id = ?').get(Number(id)),

  createDashboard: ({ name, widgets, createdBy = null }) => db.prepare(`
    INSERT INTO dashboards (name, widgets_json, created_by)
    VALUES (?, ?, ?)
  `).run(name, JSON.stringify(widgets || []), createdBy),

  updateDashboard: (id, { name, widgets }) => db.prepare(`
    UPDATE dashboards
    SET name = ?, widgets_json = ?, updated_at = CURRENT_TIMESTAMP
    WHERE id = ?
  `).run(name, JSON.stringify(widgets || []), Number(id)),

  deleteDashboard: (id) => db.prepare('DELETE FROM dashboards WHERE id = ?').run(Number(id)),

  verifyApiKey: (rawKey) => {
    if (!rawKey) {
      return null;
    }
    return db.prepare(`
      SELECT id, name, role, enabled
      FROM api_keys
      WHERE key_hash = ?
    `).get(hashApiKey(rawKey));
  },

  // ── Flow Records ──────────────────────────────────────────────

  insertFlowRecord: (record) => {
    const stmt = db.prepare(`
      INSERT INTO flow_records
        (collector_type, src_ip, dst_ip, src_port, dst_port, protocol, protocol_name,
         bytes, packets, flow_start, flow_end, input_interface, output_interface,
         tcp_flags, tos, src_as, dst_as, exporter_ip)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `);
    return stmt.run(
      record.collector_type,
      record.src_ip,
      record.dst_ip,
      record.src_port || null,
      record.dst_port || null,
      record.protocol || null,
      record.protocol_name || null,
      record.bytes || 0,
      record.packets || 0,
      record.flow_start || null,
      record.flow_end || null,
      record.input_interface || null,
      record.output_interface || null,
      record.tcp_flags || null,
      record.tos || null,
      record.src_as || null,
      record.dst_as || null,
      record.exporter_ip || null
    );
  },

  insertFlowBatch: (records) => {
    const stmt = db.prepare(`
      INSERT INTO flow_records
        (collector_type, src_ip, dst_ip, src_port, dst_port, protocol, protocol_name,
         bytes, packets, flow_start, flow_end, input_interface, output_interface,
         tcp_flags, tos, src_as, dst_as, exporter_ip)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `);
    const tx = db.transaction((rows) => {
      for (const r of rows) {
        stmt.run(
          r.collector_type, r.src_ip, r.dst_ip,
          r.src_port || null, r.dst_port || null,
          r.protocol || null, r.protocol_name || null,
          r.bytes || 0, r.packets || 0,
          r.flow_start || null, r.flow_end || null,
          r.input_interface || null, r.output_interface || null,
          r.tcp_flags || null, r.tos || null,
          r.src_as || null, r.dst_as || null,
          r.exporter_ip || null
        );
      }
    });
    tx(records);
  },

  getFlowRecords: ({ from, to, srcIp, dstIp, protocol, limit = 200 }: any = {}) => {
    const clauses = [];
    const params = [];
    if (from) { clauses.push('timestamp >= ?'); params.push(from); }
    if (to) { clauses.push('timestamp <= ?'); params.push(to); }
    if (srcIp) { clauses.push('src_ip = ?'); params.push(srcIp); }
    if (dstIp) { clauses.push('dst_ip = ?'); params.push(dstIp); }
    if (protocol) { clauses.push('protocol_name = ?'); params.push(protocol); }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return db.prepare(`
      SELECT * FROM flow_records ${where}
      ORDER BY timestamp DESC LIMIT ?
    `).all(...params, Number(limit));
  },

  getTopTalkers: ({ from, to, limit = 10, direction = 'src' }: any = {}) => {
    const ipCol = direction === 'dst' ? 'dst_ip' : 'src_ip';
    const clauses = [];
    const params = [];
    if (from) { clauses.push('timestamp >= ?'); params.push(from); }
    if (to) { clauses.push('timestamp <= ?'); params.push(to); }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return db.prepare(`
      SELECT ${ipCol} AS ip,
             SUM(bytes) AS bytes,
             SUM(packets) AS packets,
             COUNT(*) AS flows
      FROM flow_records ${where}
      GROUP BY ${ipCol}
      ORDER BY bytes DESC
      LIMIT ?
    `).all(...params, Number(limit));
  },

  getProtocolDistribution: ({ from, to }: any = {}) => {
    const clauses = [];
    const params = [];
    if (from) { clauses.push('timestamp >= ?'); params.push(from); }
    if (to) { clauses.push('timestamp <= ?'); params.push(to); }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    return db.prepare(`
      SELECT protocol_name,
             protocol,
             SUM(bytes) AS bytes,
             SUM(packets) AS packets,
             COUNT(*) AS flows
      FROM flow_records ${where}
      GROUP BY COALESCE(protocol_name, protocol)
      ORDER BY bytes DESC
    `).all(...params);
  },

  getFlowTimeseries: ({ from, to, bucketMinutes = 5 }: any = {}) => {
    const clauses = [];
    const params = [];
    if (from) { clauses.push('timestamp >= ?'); params.push(from); }
    if (to) { clauses.push('timestamp <= ?'); params.push(to); }
    const where = clauses.length ? `WHERE ${clauses.join(' AND ')}` : '';
    const bucketSeconds = Math.max(1, Number(bucketMinutes)) * 60;
    return db.prepare(`
      SELECT
        datetime((cast(strftime('%s', timestamp) as integer) / ${bucketSeconds}) * ${bucketSeconds}, 'unixepoch') AS bucket_time,
        SUM(bytes) AS total_bytes,
        SUM(packets) AS total_packets,
        COUNT(*) AS flow_count
      FROM flow_records ${where}
      GROUP BY bucket_time
      ORDER BY bucket_time ASC
      LIMIT 2000
    `).all(...params);
  },

  getFlowStats: () => {
    const row = db.prepare(`
      SELECT
        COUNT(*) AS total_flows,
        COALESCE(SUM(bytes), 0) AS total_bytes,
        COALESCE(SUM(packets), 0) AS total_packets,
        COUNT(DISTINCT src_ip) AS unique_sources,
        COUNT(DISTINCT dst_ip) AS unique_destinations
      FROM flow_records
    `).get();
    const collectors = db.prepare(`
      SELECT DISTINCT collector_type FROM flow_records
    `).all().map((r) => r.collector_type);
    const exporters = db.prepare(`
      SELECT DISTINCT exporter_ip FROM flow_records WHERE exporter_ip IS NOT NULL
    `).all().length;
    return { ...row, activeCollectors: exporters, collectorTypes: collectors };
  },

  // ── Port Scan Results ────────────────────────────────────────

  upsertPortScanResult: ({ deviceId, port, status, serviceGuess = null, responseTime = null }: any) => {
    return db.prepare(`
      INSERT INTO port_scan_results
        (device_id, port, status, service_guess, response_time, first_seen, last_seen, last_changed_at)
      VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
      ON CONFLICT(device_id, port) DO UPDATE SET
        status = excluded.status,
        service_guess = excluded.service_guess,
        response_time = excluded.response_time,
        last_seen = CURRENT_TIMESTAMP,
        last_changed_at = CASE
          WHEN port_scan_results.status != excluded.status THEN CURRENT_TIMESTAMP
          ELSE port_scan_results.last_changed_at
        END
    `).run(
      Number(deviceId),
      Number(port),
      status,
      serviceGuess,
      responseTime
    );
  },

  getPortScanResults: (deviceId) => db.prepare(`
    SELECT *
    FROM port_scan_results
    WHERE device_id = ?
    ORDER BY status = 'open' DESC, port ASC
  `).all(Number(deviceId)),

  // ── Capture Sessions ──────────────────────────────────────────

  createCaptureSession: (interfaceName, filter = null) => {
    return db.prepare(`
      INSERT INTO capture_sessions (interface_name, filter, status)
      VALUES (?, ?, 'running')
    `).run(interfaceName, filter);
  },

  stopCaptureSession: (id, packetCount = 0, bytesCaptured = 0, errorMessage = null) => {
    return db.prepare(`
      UPDATE capture_sessions
      SET status = CASE WHEN ? IS NOT NULL THEN 'error' ELSE 'stopped' END,
          packet_count = ?, bytes_captured = ?,
          stopped_at = CURRENT_TIMESTAMP, error_message = ?
      WHERE id = ?
    `).run(errorMessage, packetCount, bytesCaptured, errorMessage, Number(id));
  },

  getCaptureSession: (id) => db.prepare('SELECT * FROM capture_sessions WHERE id = ?').get(Number(id)),

  getCaptureSessions: (limit = 50) => db.prepare(
    'SELECT * FROM capture_sessions ORDER BY started_at DESC LIMIT ?'
  ).all(Number(limit)),

  updateCaptureSessionStats: (id, packetCount, bytesCaptured) => {
    return db.prepare(`
      UPDATE capture_sessions SET packet_count = ?, bytes_captured = ? WHERE id = ?
    `).run(packetCount, bytesCaptured, Number(id));
  },

  /**
   * Fetch metrics in two time windows for trend comparison.
   * Returns { recent: Metric[], baseline: Metric[] } where:
   *   recent   = last `windowHours` hours
   *   baseline = the `windowHours` before that
   */
  getMetricsForTrend: (windowHours = 1) => {
    const now = Date.now();
    const recentSince = new Date(now - windowHours * 60 * 60 * 1000)
      .toISOString().slice(0, 19).replace('T', ' ');
    const baselineSince = new Date(now - windowHours * 2 * 60 * 60 * 1000)
      .toISOString().slice(0, 19).replace('T', ' ');

    const recent = db.prepare(`
      SELECT m.*, d.name AS device_name, d.type AS device_type, d.host, d.protocol
      FROM metrics m
      JOIN devices d ON d.id = m.device_id
      WHERE m.timestamp >= ?
      ORDER BY m.timestamp DESC
      LIMIT 10000
    `).all(recentSince);

    const baseline = db.prepare(`
      SELECT m.*, d.name AS device_name, d.type AS device_type, d.host, d.protocol
      FROM metrics m
      JOIN devices d ON d.id = m.device_id
      WHERE m.timestamp >= ? AND m.timestamp < ?
      ORDER BY m.timestamp DESC
      LIMIT 10000
    `).all(baselineSince, recentSince);

    return { recent, baseline };
  },

  /**
   * Fetch metrics for a specific historical window (used for history timeline).
   */
  getMetricsInWindow: (fromIso, toIso) => {
    return db.prepare(`
      SELECT m.*, d.name AS device_name, d.type AS device_type, d.host, d.protocol
      FROM metrics m
      JOIN devices d ON d.id = m.device_id
      WHERE m.timestamp >= ? AND m.timestamp < ?
      ORDER BY m.timestamp DESC
      LIMIT 10000
    `).all(fromIso, toIso);
  }
};

export {};
