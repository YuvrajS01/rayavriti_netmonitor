// ─── Data Generators ──────────────────────────────────────────
// Functions that produce realistic simulated metrics for every
// protocol type, plus NetFlow traffic records.

import type { DeviceProfile } from './topology';

// ── Helpers ──────────────────────────────────────────────────

/** Box-Muller gaussian random: mean ± stddev */
function gaussian(mean: number, stddev: number): number {
  const u1 = Math.random() || 0.0001;
  const u2 = Math.random();
  const z = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
  return mean + z * stddev;
}

/** Clamp a number between min and max */
function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

/** Weighted random selection */
function weightedRandom<T>(items: T[], weights: number[]): T {
  const total = weights.reduce((s, w) => s + w, 0);
  let r = Math.random() * total;
  for (let i = 0; i < items.length; i++) {
    r -= weights[i];
    if (r <= 0) return items[i];
  }
  return items[items.length - 1];
}

/** Sinusoidal value that oscillates based on elapsed time — simulates
 *  natural load patterns (e.g. CPU peaking at "midday"). */
function sinusoidalLoad(
  elapsedMs: number,
  range: [number, number],
  periodMs: number = 300_000, // 5 min cycle by default
  noise: number = 3
): number {
  const phase = (elapsedMs / periodMs) * 2 * Math.PI;
  const [lo, hi] = range;
  const mid = (lo + hi) / 2;
  const amplitude = (hi - lo) / 2;
  return clamp(mid + amplitude * Math.sin(phase) + gaussian(0, noise), lo - 5, hi + 5);
}

// ── Metric result shape ─────────────────────────────────────

export interface SimulatedMetric {
  status: 'up' | 'down' | 'warning' | 'degraded' | 'ok';
  responseTime: number | null;
  value: number;
  message: string;
}

// ── Per-protocol generators ─────────────────────────────────

/**
 * Generate a ping metric with Gaussian-distributed response times.
 */
export function generatePingMetric(
  profile: DeviceProfile,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  // Determine if device is up
  const roll = Math.random();
  let status: SimulatedMetric['status'];

  if (overrideStatus) {
    status = overrideStatus;
  } else if (roll > profile.uptimeProbability) {
    status = 'down';
  } else if (roll > profile.uptimeProbability - profile.degradedProbability) {
    status = 'warning';
  } else {
    status = 'up';
  }

  if (status === 'down') {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'Ping unreachable',
    };
  }

  const [lo, hi] = profile.latencyRange;
  const mean = (lo + hi) / 2;
  let responseTime = Math.round(gaussian(mean, profile.latencyJitter) * 100) / 100;
  responseTime = clamp(responseTime, lo * 0.5, hi * 3);

  if (status === 'warning') {
    responseTime = Math.round(responseTime * (2 + Math.random() * 2));
  }

  return {
    status,
    responseTime,
    value: 1,
    message: status === 'warning'
      ? `Ping reachable (high latency: ${responseTime}ms)`
      : 'Ping reachable',
  };
}

/**
 * Generate an HTTP metric with weighted status codes.
 */
export function generateHttpMetric(
  profile: DeviceProfile,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  const weights = profile.httpStatusWeights || { 200: 85, 301: 5, 503: 5, 0: 5 };
  const codes = Object.keys(weights).map(Number);
  const w = Object.values(weights);

  let httpCode: number;

  if (overrideStatus === 'down') {
    httpCode = 0; // timeout
  } else if (overrideStatus === 'warning') {
    httpCode = 503;
  } else if (overrideStatus) {
    httpCode = 200;
  } else {
    httpCode = weightedRandom(codes, w);
  }

  if (httpCode === 0) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'HTTP error: The operation was aborted',
    };
  }

  const [lo, hi] = profile.latencyRange;
  const mean = (lo + hi) / 2;
  let responseTime = Math.round(gaussian(mean, profile.latencyJitter));
  responseTime = clamp(responseTime, lo * 0.5, hi * 3);

  if (httpCode >= 500) {
    return {
      status: 'degraded',
      responseTime: Math.round(responseTime * (1.5 + Math.random())),
      value: httpCode,
      message: `HTTP ${httpCode}`,
    };
  }

  return {
    status: httpCode >= 200 && httpCode < 400 ? 'up' : 'degraded',
    responseTime,
    value: httpCode,
    message: `HTTP ${httpCode}`,
  };
}

/**
 * Generate a full SNMP metric with CPU, memory, disk, and interface counters.
 * Counters always increase; utilization follows sinusoidal patterns.
 */
// Persistent counter state per-device (survives across calls)
const interfaceCounters = new Map<string, { inOctets: number; outOctets: number }[]>();

export function generateSnmpMetric(
  profile: DeviceProfile,
  elapsedMs: number,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  if (overrideStatus === 'down') {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'SNMP error: Request timed out',
    };
  }

  const roll = Math.random();
  if (!overrideStatus && roll > profile.uptimeProbability) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'SNMP error: Request timed out',
    };
  }

  const sp = profile.snmpProfile!;
  const cpu = Math.round(sinusoidalLoad(elapsedMs, sp.cpuRange, 240_000, 5) * 10) / 10;
  const memPercent = Math.round(sinusoidalLoad(elapsedMs, sp.memoryRange, 360_000, 3) * 10) / 10;
  const diskPercent = Math.round(sinusoidalLoad(elapsedMs, sp.diskRange, 600_000, 1) * 10) / 10;

  const totalMemGb = profile.type === 'switch' ? 2 : profile.type === 'printer' ? 0.5 : 8;
  const totalDiskGb = profile.type === 'switch' ? 4 : profile.type === 'printer' ? 1 : 100;
  const uptimeSeconds = Math.round(elapsedMs / 1000) + 86400 * 30; // pretend 30 days + sim time

  // Build interface counters (always increasing)
  if (!interfaceCounters.has(profile.name)) {
    const ifaces = [];
    for (let i = 0; i < sp.interfaceCount; i++) {
      ifaces.push({
        inOctets: Math.floor(Math.random() * 1_000_000_000),
        outOctets: Math.floor(Math.random() * 800_000_000),
      });
    }
    interfaceCounters.set(profile.name, ifaces);
  }

  const ifaces = interfaceCounters.get(profile.name)!;
  const interfaces = ifaces.map((iface, idx) => {
    // Increment counters
    const trafficMultiplier = idx < 4 ? 1 : 0.3; // first 4 ports are busier
    iface.inOctets += Math.floor(Math.random() * 500_000 * trafficMultiplier);
    iface.outOctets += Math.floor(Math.random() * 400_000 * trafficMultiplier);
    const speed = idx < 4 ? 1_000_000_000 : 100_000_000; // 1G vs 100M
    return {
      index: idx + 1,
      name: idx < 2 ? `GigabitEthernet0/${idx}` : `FastEthernet0/${idx}`,
      inOctets: iface.inOctets,
      outOctets: iface.outOctets,
      speed,
      operStatus: Math.random() > 0.03 ? 1 : 2, // 3% chance of down
    };
  }).slice(0, 12); // cap at 12 shown

  const resourceInfo = {
    cpu: { usage: cpu, cores: profile.type === 'switch' ? 1 : 4 },
    memory: {
      used: Math.round(totalMemGb * memPercent / 100 * 10) / 10,
      total: totalMemGb,
      percent: memPercent,
    },
    disk: {
      used: Math.round(totalDiskGb * diskPercent / 100 * 10) / 10,
      total: totalDiskGb,
      percent: diskPercent,
    },
    uptime: uptimeSeconds,
    interfaces,
  };

  let status: SimulatedMetric['status'] = 'up';
  if (overrideStatus === 'warning' || cpu > 90 || memPercent > 95 || diskPercent > 95) {
    status = 'warning';
  }

  const [lo, hi] = profile.latencyRange;
  const mean = (lo + hi) / 2;
  let responseTime = Math.round(gaussian(mean, profile.latencyJitter));
  responseTime = clamp(responseTime, Math.max(1, lo * 0.5), hi * 2);

  return {
    status,
    responseTime,
    value: memPercent || cpu || 0,
    message: JSON.stringify(resourceInfo),
  };
}

/**
 * Generate a system metric matching the system.ts collector output format.
 */
export function generateSystemMetric(
  profile: DeviceProfile,
  elapsedMs: number,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  if (overrideStatus === 'down') {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'System unreachable',
    };
  }

  const sp = profile.systemProfile!;
  const cpuUsage = Math.round(sinusoidalLoad(elapsedMs, sp.cpuRange, 180_000, 4) * 10) / 10;
  const memPercent = Math.round(sinusoidalLoad(elapsedMs, sp.memoryRange, 300_000, 2) * 10) / 10;
  const diskPercent = Math.round(sinusoidalLoad(elapsedMs, sp.diskRange, 900_000, 1) * 10) / 10;

  const totalMemGb = 32;
  const totalDiskGb = 500;

  const systemInfo = {
    cpu: { usage: cpuUsage, cores: 8, model: 'Intel Xeon E5-2680 v4' },
    memory: {
      used: Math.round(totalMemGb * memPercent / 100 * 10) / 10,
      total: totalMemGb,
      percent: memPercent,
    },
    disk: {
      used: Math.round(totalDiskGb * diskPercent / 100 * 10) / 10,
      total: totalDiskGb,
      percent: diskPercent,
    },
    uptime: Math.round(elapsedMs / 1000) + 86400 * 45,
    loadAvg: [
      Math.round(cpuUsage / 12.5 * 100) / 100,
      Math.round(cpuUsage / 14 * 100) / 100,
      Math.round(cpuUsage / 16 * 100) / 100,
    ],
  };

  let status: SimulatedMetric['status'] = 'ok';
  if (overrideStatus === 'warning' || cpuUsage > 90 || memPercent > 95) {
    status = 'warning';
  }

  return {
    status,
    responseTime: 0,
    value: memPercent,
    message: JSON.stringify(systemInfo),
  };
}

/**
 * Generate a port check metric.
 */
export function generatePortMetric(
  profile: DeviceProfile,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  const roll = Math.random();
  let isUp: boolean;

  if (overrideStatus === 'down') {
    isUp = false;
  } else if (overrideStatus) {
    isUp = true;
  } else {
    isUp = roll <= profile.uptimeProbability;
  }

  if (!isUp) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: `Port check failed: connect ECONNREFUSED ${profile.host}:${profile.port}`,
    };
  }

  const [lo, hi] = profile.latencyRange;
  const mean = (lo + hi) / 2;
  let responseTime = Math.round(gaussian(mean, profile.latencyJitter));
  responseTime = clamp(responseTime, Math.max(1, lo * 0.5), hi * 2);

  return {
    status: 'up',
    responseTime,
    value: 1,
    message: `Port ${profile.port} open`,
  };
}

// ── NetFlow Record Generator ────────────────────────────────

export interface SimulatedFlowRecord {
  collector_type: string;
  src_ip: string;
  dst_ip: string;
  src_port: number;
  dst_port: number;
  protocol: number;
  protocol_name: string;
  bytes: number;
  packets: number;
  flow_start: string | null;
  flow_end: string | null;
  input_interface: number | null;
  output_interface: number | null;
  tcp_flags: number | null;
  tos: number | null;
  src_as: number | null;
  dst_as: number | null;
  exporter_ip: string | null;
}

const COMMON_PORTS = [80, 443, 8080, 53, 22, 25, 587, 3306, 5432, 8443, 3000, 9090];
const EXTERNAL_IPS = [
  '203.0.113.10', '198.51.100.50', '192.0.2.100', '203.0.113.200',
  '198.51.100.75', '192.0.2.25', '203.0.113.44', '198.51.100.120',
];

const PROTOCOL_TABLE: Array<{ proto: number; name: string; weight: number }> = [
  { proto: 6, name: 'TCP', weight: 60 },
  { proto: 17, name: 'UDP', weight: 25 },
  { proto: 1, name: 'ICMP', weight: 8 },
  { proto: 47, name: 'GRE', weight: 3 },
  { proto: 89, name: 'OSPF', weight: 2 },
  { proto: 132, name: 'SCTP', weight: 2 },
];

/**
 * Generate a batch of realistic NetFlow records representing inter-device traffic.
 */
export function generateFlowBatch(
  deviceIps: string[],
  batchSize: number,
  trafficMultiplier: number = 1
): SimulatedFlowRecord[] {
  const records: SimulatedFlowRecord[] = [];
  const now = new Date();

  for (let i = 0; i < batchSize; i++) {
    // Pick protocol
    const protoEntry = weightedRandom(
      PROTOCOL_TABLE,
      PROTOCOL_TABLE.map((p) => p.weight)
    );

    // ~70% internal-to-internal, ~30% internal-to-external
    const isExternal = Math.random() < 0.3;
    const srcIp = deviceIps[Math.floor(Math.random() * deviceIps.length)];
    const dstIp = isExternal
      ? EXTERNAL_IPS[Math.floor(Math.random() * EXTERNAL_IPS.length)]
      : deviceIps[Math.floor(Math.random() * deviceIps.length)];

    // Ports
    let srcPort = 1024 + Math.floor(Math.random() * 64000);
    let dstPort = COMMON_PORTS[Math.floor(Math.random() * COMMON_PORTS.length)];
    if (protoEntry.proto === 1 || protoEntry.proto === 89) {
      srcPort = 0;
      dstPort = 0;
    }

    // Traffic volume
    const baseBytes = protoEntry.proto === 6
      ? 500 + Math.floor(Math.random() * 50000)  // TCP: 500B - 50KB
      : protoEntry.proto === 17
        ? 64 + Math.floor(Math.random() * 2000)   // UDP: 64B - 2KB
        : 64 + Math.floor(Math.random() * 500);   // ICMP/other: 64-564B

    const bytes = Math.round(baseBytes * trafficMultiplier);
    const packets = Math.max(1, Math.round(bytes / (40 + Math.random() * 1400)));

    // TCP flags
    let tcpFlags: number | null = null;
    if (protoEntry.proto === 6) {
      const flagSets = [0x02, 0x12, 0x10, 0x18, 0x11, 0x04]; // SYN, SYN-ACK, ACK, PSH-ACK, FIN-ACK, RST
      tcpFlags = flagSets[Math.floor(Math.random() * flagSets.length)];
    }

    const flowDuration = 1000 + Math.floor(Math.random() * 30000); // 1-30s
    const flowStart = new Date(now.getTime() - flowDuration);

    records.push({
      collector_type: Math.random() < 0.7 ? 'netflow_v9' : 'netflow_v5',
      src_ip: srcIp,
      dst_ip: dstIp,
      src_port: srcPort,
      dst_port: dstPort,
      protocol: protoEntry.proto,
      protocol_name: protoEntry.name,
      bytes,
      packets,
      flow_start: flowStart.toISOString(),
      flow_end: now.toISOString(),
      input_interface: Math.floor(Math.random() * 8) + 1,
      output_interface: Math.floor(Math.random() * 8) + 1,
      tcp_flags: tcpFlags,
      tos: Math.random() < 0.8 ? 0 : Math.floor(Math.random() * 4) * 32,
      src_as: isExternal ? 15169 + Math.floor(Math.random() * 50000) : null,
      dst_as: isExternal ? 13335 + Math.floor(Math.random() * 30000) : null,
      exporter_ip: '10.0.0.1',
    });
  }

  return records;
}

/**
 * Master dispatcher: generate the right metric type for a given device profile.
 */
export function generateMetricForDevice(
  profile: DeviceProfile,
  elapsedMs: number,
  overrideStatus?: 'up' | 'down' | 'warning'
): SimulatedMetric {
  switch (profile.protocol) {
    case 'ping':
      return generatePingMetric(profile, overrideStatus);
    case 'http':
      return generateHttpMetric(profile, overrideStatus);
    case 'snmp':
      return generateSnmpMetric(profile, elapsedMs, overrideStatus);
    case 'system':
      return generateSystemMetric(profile, elapsedMs, overrideStatus);
    case 'port':
      return generatePortMetric(profile, overrideStatus);
    default:
      return generatePingMetric(profile, overrideStatus);
  }
}
