#!/usr/bin/env ts-node
// ─── Rayavriti NetMonitor  —  Network Simulator ─────────────
// Standalone script that seeds 25 virtual network devices and
// continuously injects realistic telemetry into the running
// server, exercising every dashboard feature.
//
// Usage:
//   npx ts-node simulator/run.ts [options]
//
// Options:
//   --duration=600      Duration in seconds (default: 600 / 10 min)
//   --speed=1           Speed multiplier (2 = intervals halved)
//   --scenario=mixed    Preset: stable | degraded | outage | mixed
//   --write-mode=auto   Write mode: auto | api | db (default: auto)
//   --no-netflow        Disable NetFlow generation
//   --no-scenarios      Disable scripted failure scenarios
// ──────────────────────────────────────────────────────────────

import { ENTERPRISE_TOPOLOGY, type DeviceProfile } from './topology';
import { generateMetricForDevice, generateFlowBatch, type SimulatedMetric } from './generators';
import { ScenarioEngine } from './scenarios';

// ── Parse CLI arguments ─────────────────────────────────────

function parseArgs() {
  const args: Record<string, string> = {};
  for (const arg of process.argv.slice(2)) {
    if (arg.startsWith('--')) {
      const [key, val] = arg.slice(2).split('=');
      args[key] = val ?? 'true';
    }
  }
  return {
    duration: Number(args.duration ?? 600),
    speed: Number(args.speed ?? 1),
    scenario: args.scenario ?? 'mixed',
    writeMode: args['write-mode'] ?? process.env.SIMULATOR_WRITE_MODE ?? 'auto',
    netflow: args['no-netflow'] !== 'true',
    scenarios: args['no-scenarios'] !== 'true',
  };
}

// ── Load config ─────────────────────────────────────────────

const path = require('path');
const fs = require('fs');

let config: any = {};
const configPath = path.resolve(__dirname, 'config.json');
if (fs.existsSync(configPath)) {
  config = JSON.parse(fs.readFileSync(configPath, 'utf-8'));
}

const serverUrl = process.env.SIMULATOR_API_URL || config.server?.apiUrl || 'http://localhost:3000';
const simConfig = config.simulation || {};

// ── Database access (shared SQLite with the server) ─────────

const originalCwd = process.cwd();
let db: any = null;

function loadDbModule() {
  if (db) return db;

  // Resolve database module relative to the server source
  const dbModulePath = path.resolve(__dirname, '..', 'server', 'src', 'services', 'database');
  try {
    // Change working directory so the DB module resolves ./data correctly
    process.chdir(path.resolve(__dirname, '../server'));
    db = require(dbModulePath).default || require(dbModulePath);
    process.chdir(originalCwd);
    return db;
  } catch (err) {
    process.chdir(originalCwd);
    throw new Error(
      `Could not load database module from ${dbModulePath}. ` +
      `Make sure the server source is available and dependencies are installed. ` +
      `Error: ${(err as Error).message}`
    );
  }
}

// ── HTTP client for API operations ──────────────────────────

async function apiRequest(method: string, endpoint: string, body?: any, token?: string) {
  const url = `${serverUrl}${endpoint}`;
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const options: RequestInit = { method, headers };
  if (body) options.body = JSON.stringify(body);

  const res = await fetch(url, options);
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`API ${method} ${endpoint} → ${res.status}: ${text.slice(0, 200)}`);
  }
  return res.json();
}

async function tryApiRequest(method: string, endpoint: string, body?: any, token?: string) {
  try {
    return await apiRequest(method, endpoint, body, token);
  } catch {
    return null;
  }
}

// ── Color helpers for CLI output ────────────────────────────

const C = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m',
  magenta: '\x1b[35m',
  blue: '\x1b[34m',
  white: '\x1b[37m',
  bgBlack: '\x1b[40m',
};

function log(prefix: string, color: string, msg: string) {
  const ts = new Date().toLocaleTimeString('en-US', { hour12: false });
  console.log(`${C.dim}${ts}${C.reset} ${color}[${prefix}]${C.reset} ${msg}`);
}

// ── Device seeding ──────────────────────────────────────────

interface SeededDevice {
  profile: DeviceProfile;
  dbId: number;
  sensorId: number | null;
}

async function seedDevices(token: string, mode: 'api' | 'db'): Promise<SeededDevice[]> {
  const seeded: SeededDevice[] = [];
  const existingDevices = mode === 'api'
    ? ((await apiRequest('GET', '/api/devices', undefined, token)).data || [])
    : loadDbModule().getDevices();

  for (const profile of ENTERPRISE_TOPOLOGY) {
    // Check if device already exists (by name)
    let existing = existingDevices.find(
      (d: any) => d.name === profile.name
    );

    if (!existing) {
      // Create via API so it goes through all the normal hooks
      try {
        const res = await apiRequest('POST', '/api/devices', {
          name: profile.name,
          type: profile.type,
          host: profile.host,
          port: profile.port,
          protocol: profile.protocol,
          interval_seconds: profile.intervalSeconds,
          snmpCommunity: profile.protocol === 'snmp' ? 'simulated' : undefined,
          snmpVersion: profile.protocol === 'snmp' ? '2c' : undefined,
        }, token);

        existing = res.data;
        log('Seed', C.green, `Created device: ${C.bold}${profile.name}${C.reset} (${profile.type}/${profile.protocol} @ ${profile.host})`);
      } catch (err) {
        // If API fails (e.g. server not running), try direct DB insert
        if (mode === 'api') {
          throw err;
        }

        log('Seed', C.yellow, `API unavailable, using direct DB insert for ${profile.name}`);
        const localDb = loadDbModule();
        const result = localDb.addDevice({
          name: profile.name,
          type: profile.type,
          host: profile.host,
          port: profile.port,
          protocol: profile.protocol,
          interval: profile.intervalSeconds,
          snmpCommunity: profile.protocol === 'snmp' ? 'simulated' : undefined,
          snmpVersion: profile.protocol === 'snmp' ? '2c' : undefined,
        });
        existing = localDb.getDevice(result.lastInsertRowid);
      }
    } else {
      log('Seed', C.dim, `Device exists: ${profile.name} (id=${existing.id})`);
    }

    // Find associated sensor
    const sensor = mode === 'db'
      ? loadDbModule().getPrimarySensorForDevice(existing.id)
      : null;

    seeded.push({
      profile,
      dbId: existing.id,
      sensorId: sensor?.id || null,
    });
  }

  return seeded;
}

// ── Authentication ──────────────────────────────────────────

async function authenticate(): Promise<string> {
  const creds = {
    username: process.env.ADMIN_USERNAME || config.server?.credentials?.username || 'admin',
    password: process.env.ADMIN_PASSWORD || config.server?.credentials?.password || 'admin123',
  };
  try {
    const res = await apiRequest('POST', '/api/auth/login', creds);
    log('Auth', C.green, `Authenticated as ${C.bold}${creds.username}${C.reset}`);
    return res.data?.token || res.token;
  } catch {
    log('Auth', C.yellow, 'API auth failed — will use direct DB operations only');
    return '';
  }
}

// ── Metric injection ────────────────────────────────────────

function injectMetricDb(device: SeededDevice, metric: SimulatedMetric): void {
  loadDbModule().recordMetric(
    device.dbId,
    metric.status,
    metric.responseTime,
    metric.value,
    metric.message,
    device.sensorId
  );
}

async function injectMetricsApi(metrics: Array<{ device: SeededDevice; metric: SimulatedMetric }>, token: string): Promise<void> {
  await apiRequest('POST', '/api/simulator/metrics', {
    metrics: metrics.map(({ device, metric }) => ({
      deviceId: device.dbId,
      deviceName: device.profile.name,
      sensorId: device.sensorId,
      status: metric.status,
      responseTime: metric.responseTime,
      value: metric.value,
      message: metric.message,
    })),
  }, token);
}

// ── Alert creation ──────────────────────────────────────────

function getAlertInfo(device: SeededDevice, metric: SimulatedMetric) {
  let severity: string | null = null;
  let message: string | null = null;

  if (metric.status === 'down') {
    severity = 'critical';
    message = `${device.profile.name} is down (${device.profile.protocol})`;
  } else if (metric.status === 'degraded' || metric.status === 'warning') {
    if (metric.responseTime && metric.responseTime > 500) {
      severity = 'warning';
      message = `${device.profile.name} latency high (${metric.responseTime}ms)`;
    }
  }

  return severity && message ? { severity, message } : null;
}

function getAlertKey(deviceId: number, message: string) {
  return `${deviceId}:${message}`;
}

function checkAndCreateAlertDb(device: SeededDevice, metric: SimulatedMetric): boolean {
  const alert = getAlertInfo(device, metric);
  if (!alert) return false;

  // Don't duplicate active alerts
  const localDb = loadDbModule();
  const existing = localDb.findActiveAlert(device.dbId, alert.message);
  if (existing) return false;

  localDb.createAlert(device.dbId, alert.severity, alert.message);
  const icon = alert.severity === 'critical' ? '🔴' : '🟡';
  log('Alert', alert.severity === 'critical' ? C.red : C.yellow,
    `${icon} ${alert.severity.toUpperCase()}: ${alert.message}`);
  return true;
}

async function loadActiveAlertKeys(token: string): Promise<Set<string>> {
  const res = await tryApiRequest('GET', '/api/alerts?status=active&limit=500', undefined, token);
  const alerts = Array.isArray(res?.data) ? res.data : [];
  return new Set(alerts.map((a: any) => getAlertKey(Number(a.device_id), String(a.message))));
}

async function checkAndCreateAlertApi(
  device: SeededDevice,
  metric: SimulatedMetric,
  token: string,
  activeAlertKeys: Set<string>
): Promise<boolean> {
  const alert = getAlertInfo(device, metric);
  if (!alert) return false;

  const key = getAlertKey(device.dbId, alert.message);
  if (activeAlertKeys.has(key)) return false;

  await apiRequest('POST', '/api/v1/alerts', {
    deviceId: device.dbId,
    severity: alert.severity,
    message: alert.message,
    status: 'active',
  }, token);
  activeAlertKeys.add(key);

  const icon = alert.severity === 'critical' ? '🔴' : '🟡';
  log('Alert', alert.severity === 'critical' ? C.red : C.yellow,
    `${icon} ${alert.severity.toUpperCase()}: ${alert.message}`);
  return true;
}

// ── Flow injection ──────────────────────────────────────────

async function injectFlows(deviceIps: string[], batchSize: number, multiplier: number, mode: 'api' | 'db', token: string): Promise<void> {
  const records = generateFlowBatch(deviceIps, batchSize, multiplier);
  try {
    if (mode === 'api') {
      await apiRequest('POST', '/api/simulator/flows', { flows: records }, token);
    } else {
      loadDbModule().insertFlowBatch(records);
    }
  } catch (err) {
    log('Flow', C.red, `DB insert error: ${(err as Error).message}`);
  }
}

// ── Gradual degradation helper ──────────────────────────────
// For DB-Primary: linearly increase response time from 50ms to 2000ms

function applyGradualDegradation(
  metric: SimulatedMetric,
  factor: number
): SimulatedMetric {
  const baseResponse = 50;
  const peakResponse = 2000;
  const interpolated = Math.round(baseResponse + (peakResponse - baseResponse) * factor);

  return {
    ...metric,
    responseTime: interpolated,
    status: factor > 0.6 ? 'warning' : metric.status,
    message: metric.message.replace(
      /}$/,
      `,"degradation_factor":${Math.round(factor * 100)}}`
    ),
  };
}

// ── Main ────────────────────────────────────────────────────

async function main() {
  const args = parseArgs();

  console.log('');
  console.log(`${C.bold}${C.cyan}╔══════════════════════════════════════════════════════════╗${C.reset}`);
  console.log(`${C.bold}${C.cyan}║${C.reset}  ${C.bold}${C.green}Rayavriti NetMonitor${C.reset} — ${C.bold}Network Simulator${C.reset}              ${C.cyan}║${C.reset}`);
  console.log(`${C.bold}${C.cyan}╚══════════════════════════════════════════════════════════╝${C.reset}`);
  console.log('');
  console.log(`  ${C.dim}Duration:${C.reset}    ${args.duration}s`);
  console.log(`  ${C.dim}Speed:${C.reset}       ${args.speed}x`);
  console.log(`  ${C.dim}Scenario:${C.reset}    ${args.scenario}`);
  console.log(`  ${C.dim}Write mode:${C.reset}  ${args.writeMode}`);
  console.log(`  ${C.dim}NetFlow:${C.reset}     ${args.netflow ? 'enabled' : 'disabled'}`);
  console.log(`  ${C.dim}Scenarios:${C.reset}   ${args.scenarios ? 'enabled' : 'disabled'}`);
  console.log(`  ${C.dim}Server:${C.reset}      ${serverUrl}`);
  console.log(`  ${C.dim}Devices:${C.reset}     ${ENTERPRISE_TOPOLOGY.length}`);
  console.log('');

  // Authenticate
  const token = await authenticate();
  let writeMode: 'api' | 'db' = 'db';

  if (args.writeMode === 'api') {
    if (!token) {
      throw new Error('API write mode requires successful authentication. Check ADMIN_USERNAME/ADMIN_PASSWORD.');
    }
    writeMode = 'api';
  } else if (args.writeMode === 'db') {
    loadDbModule();
    writeMode = 'db';
  } else {
    const health = token ? await tryApiRequest('GET', '/api/devices', undefined, token) : null;
    if (health) {
      writeMode = 'api';
      log('Init', C.green, 'Using API writes (Docker-compatible)');
    } else {
      loadDbModule();
      writeMode = 'db';
      log('Init', C.yellow, 'Using direct DB writes (local fallback)');
    }
  }

  // Seed devices
  log('Init', C.cyan, `Seeding ${ENTERPRISE_TOPOLOGY.length} devices...`);
  const devices = await seedDevices(token, writeMode);
  log('Init', C.green, `✓ ${devices.length} devices ready`);
  const activeAlertKeys = writeMode === 'api'
    ? await loadActiveAlertKeys(token)
    : new Set<string>();

  // Scenario engine
  const scenarioEngine = args.scenarios
    ? new ScenarioEngine(args.scenario)
    : new ScenarioEngine('stable');

  // Collect all device IPs for flow generation
  const deviceIps = devices.map((d) => d.profile.host);

  // Timing
  const metricIntervalMs = Math.round(
    (simConfig.metricIntervalMs || 10000) / args.speed
  );
  const flowIntervalMs = Math.round(
    (simConfig.flowIntervalMs || 2000) / args.speed
  );
  const flowBatchSize = simConfig.flowBatchSize || 50;

  const startTime = Date.now();
  const endTime = startTime + args.duration * 1000;

  let metricCycleCount = 0;
  let flowCycleCount = 0;
  let totalMetricsInjected = 0;
  let totalFlowsInjected = 0;
  let totalAlertsCreated = 0;

  // ── Metric generation loop ─────────────────────────────
  const metricTimer = setInterval(() => {
    void (async () => {
    const now = Date.now();
    if (now >= endTime) return;

    const elapsedMs = now - startTime;
    const elapsedSec = Math.round(elapsedMs / 1000);
    metricCycleCount++;

    // Tick the scenario engine
    const { triggered, ended } = scenarioEngine.tick(elapsedSec);
    for (const event of triggered) {
      log('Scenario', C.magenta, `⚡ ${event.description}`);
    }
    for (const event of ended) {
      log('Scenario', C.green, `✓ RECOVERED: ${event.description.split(':')[0]}`);
    }

    // Generate and inject metrics for each device
    let cycleDown = 0;
    let cycleWarn = 0;
    let cycleUp = 0;
    const cycleMetrics: Array<{ device: SeededDevice; metric: SimulatedMetric }> = [];

    for (const device of devices) {
      // Check for time-of-day offline (PCs)
      if (device.profile.offlineHours) {
        const currentHour = new Date().getHours();
        if (device.profile.offlineHours.includes(currentHour)) {
          const metric: SimulatedMetric = {
            status: 'down',
            responseTime: null,
            value: 0,
            message: 'Device offline (outside working hours)',
          };
          if (writeMode === 'db') {
            injectMetricDb(device, metric);
          } else {
            cycleMetrics.push({ device, metric });
          }
          cycleDown++;
          totalMetricsInjected++;
          continue;
        }
      }

      // Get scenario override
      let override = scenarioEngine.getDeviceOverride(device.profile.name);

      // Generate metric
      let metric = generateMetricForDevice(device.profile, elapsedMs, override);

      // Apply gradual degradation for DB-Primary
      const gradualFactor = scenarioEngine.getGradualDegradationFactor(
        device.profile.name, elapsedSec
      );
      if (gradualFactor !== null) {
        metric = applyGradualDegradation(metric, gradualFactor);
      }

      // Queue metric for API batch or inject into local database.
      if (writeMode === 'db') {
        injectMetricDb(device, metric);
      } else {
        cycleMetrics.push({ device, metric });
      }
      totalMetricsInjected++;

      // Check for alerts
      if (metric.status === 'down' || metric.status === 'warning' || metric.status === 'degraded') {
        if (writeMode === 'db') {
          if (checkAndCreateAlertDb(device, metric)) {
            totalAlertsCreated++;
          }
        } else {
          try {
            if (await checkAndCreateAlertApi(device, metric, token, activeAlertKeys)) {
              totalAlertsCreated++;
            }
          } catch (err) {
            log('Alert', C.red, `API create error: ${(err as Error).message}`);
          }
        }
      }

      // Stats
      if (metric.status === 'down') cycleDown++;
      else if (metric.status === 'warning' || metric.status === 'degraded') cycleWarn++;
      else cycleUp++;
    }

    if (writeMode === 'api' && cycleMetrics.length > 0) {
      try {
        await injectMetricsApi(cycleMetrics, token);
      } catch (err) {
        log('Metrics', C.red, `API insert error: ${(err as Error).message}`);
      }
    }

    // Cycle summary
    const statusBar = `${C.green}${cycleUp} up${C.reset} / ${C.yellow}${cycleWarn} warn${C.reset} / ${C.red}${cycleDown} down${C.reset}`;
    const remaining = Math.round((endTime - now) / 1000);
    log('Metrics', C.blue,
      `Cycle #${metricCycleCount}: ${devices.length} metrics injected  [${statusBar}]  ${C.dim}(${remaining}s remaining)${C.reset}`);

    })().catch((err) => log('Metrics', C.red, `Cycle error: ${(err as Error).message}`));
  }, metricIntervalMs);

  // ── NetFlow generation loop ────────────────────────────
  let flowTimer: NodeJS.Timeout | null = null;
  if (args.netflow) {
    flowTimer = setInterval(() => {
      const now = Date.now();
      if (now >= endTime) return;

      flowCycleCount++;
      const multiplier = scenarioEngine.getFlowMultiplier();
      const actualBatch = Math.round(flowBatchSize * (multiplier > 1 ? multiplier * 0.5 : 1));

      void injectFlows(deviceIps, actualBatch, multiplier, writeMode, token);
      totalFlowsInjected += actualBatch;

      if (flowCycleCount % 10 === 0) {
        const totalBytes = actualBatch * 5000; // rough estimate
        const formatted = totalBytes > 1_000_000
          ? `${(totalBytes / 1_000_000).toFixed(1)} MB`
          : `${(totalBytes / 1000).toFixed(1)} KB`;
        log('Flow', C.cyan,
          `Batch #${flowCycleCount}: ${actualBatch} records (~${formatted})${multiplier > 1 ? ` ${C.red}[${multiplier}x traffic spike]${C.reset}` : ''}`);
      }
    }, flowIntervalMs);
  }

  // ── Completion handler ─────────────────────────────────

  const shutdown = () => {
    clearInterval(metricTimer);
    if (flowTimer) clearInterval(flowTimer);

    console.log('');
    console.log(`${C.bold}${C.cyan}══════════════════════════════════════════════════════════${C.reset}`);
    console.log(`${C.bold}  Simulation Complete${C.reset}`);
    console.log(`${C.cyan}══════════════════════════════════════════════════════════${C.reset}`);
    console.log('');
    console.log(`  ${C.dim}Duration:${C.reset}              ${args.duration}s`);
    console.log(`  ${C.dim}Devices simulated:${C.reset}     ${devices.length}`);
    console.log(`  ${C.dim}Metric cycles:${C.reset}         ${metricCycleCount}`);
    console.log(`  ${C.dim}Total metrics:${C.reset}         ${totalMetricsInjected}`);
    console.log(`  ${C.dim}Flow batches:${C.reset}          ${flowCycleCount}`);
    console.log(`  ${C.dim}Total flows:${C.reset}           ${totalFlowsInjected}`);
    console.log(`  ${C.dim}Alerts triggered:${C.reset}      ${totalAlertsCreated}`);

    if (args.scenarios) {
      const scenarioLog = scenarioEngine.getLog();
      if (scenarioLog.length > 0) {
        console.log('');
        console.log(`  ${C.magenta}${C.bold}Scenario Events:${C.reset}`);
        for (const entry of scenarioLog) {
          console.log(`    ${C.dim}${entry}${C.reset}`);
        }
      }
    }

    console.log('');
    process.exit(0);
  };

  // End after duration
  setTimeout(shutdown, args.duration * 1000 + 500);

  // Handle Ctrl+C
  process.on('SIGINT', () => {
    log('Sim', C.yellow, 'Interrupted (SIGINT)');
    shutdown();
  });

  log('Sim', C.green,
    `${C.bold}Simulation running${C.reset} — ${args.duration}s at ${args.speed}x speed, metrics every ${metricIntervalMs}ms`);
  log('Sim', C.dim, 'Press Ctrl+C to stop early');
}

// ── Run ─────────────────────────────────────────────────────

main().catch((err) => {
  console.error(`${C.red}[Simulator] Fatal error:${C.reset}`, err);
  process.exit(1);
});
