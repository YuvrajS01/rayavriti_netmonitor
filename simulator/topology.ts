// ─── Device Topology & Behavior Profiles ──────────────────────
// Defines a 25-device enterprise network for simulation.

export interface DeviceProfile {
  name: string;
  type: 'router' | 'switch' | 'firewall' | 'server' | 'pc' | 'printer' | 'access_point';
  host: string;
  protocol: 'ping' | 'http' | 'snmp' | 'system' | 'port';
  port: number;
  intervalSeconds: number;
  subnet: string;

  // Behavior model
  latencyRange: [number, number];           // [min, max] in ms
  latencyJitter: number;                    // std deviation for gaussian noise
  uptimeProbability: number;                // 0-1, probability of being "up" per check
  degradedProbability: number;              // 0-1, probability of "warning/degraded" when not down
  httpStatusWeights?: Record<number, number>; // HTTP status code → weight
  portList?: number[];                      // For port-type devices
  snmpProfile?: {
    cpuRange: [number, number];
    memoryRange: [number, number];
    diskRange: [number, number];
    interfaceCount: number;
  };
  systemProfile?: {
    cpuRange: [number, number];
    memoryRange: [number, number];
    diskRange: [number, number];
  };
  // Time-aware: offline during these simulated "hours" (0-23)
  offlineHours?: number[];
}

/**
 * 25-device enterprise network topology.
 * IPs use non-routable 10.x.x.x and 172.16.x.x addresses so the real
 * scheduler's ping/HTTP checks will fail harmlessly—our simulator injects
 * metrics directly, overriding any scheduler-produced "down" results.
 */
export const ENTERPRISE_TOPOLOGY: DeviceProfile[] = [
  // ═══ Core Routers ═══════════════════════════════════════════
  {
    name: 'Router-Core-01',
    type: 'router',
    host: '10.0.0.1',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 15,
    subnet: 'Core',
    latencyRange: [1, 5],
    latencyJitter: 1.2,
    uptimeProbability: 0.998,
    degradedProbability: 0.005,
  },
  {
    name: 'Router-Core-02',
    type: 'router',
    host: '10.0.0.2',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 15,
    subnet: 'Core',
    latencyRange: [1, 6],
    latencyJitter: 1.5,
    uptimeProbability: 0.997,
    degradedProbability: 0.008,
  },

  // ═══ Distribution Switches ══════════════════════════════════
  {
    name: 'Switch-Dist-01',
    type: 'switch',
    host: '10.0.0.10',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 20,
    subnet: 'Core',
    latencyRange: [15, 80],
    latencyJitter: 12,
    uptimeProbability: 0.995,
    degradedProbability: 0.015,
    snmpProfile: {
      cpuRange: [10, 40],
      memoryRange: [30, 60],
      diskRange: [5, 15],
      interfaceCount: 24,
    },
  },
  {
    name: 'Switch-Dist-02',
    type: 'switch',
    host: '10.0.0.11',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 20,
    subnet: 'Core',
    latencyRange: [12, 70],
    latencyJitter: 10,
    uptimeProbability: 0.996,
    degradedProbability: 0.01,
    snmpProfile: {
      cpuRange: [8, 35],
      memoryRange: [25, 55],
      diskRange: [5, 12],
      interfaceCount: 24,
    },
  },
  {
    name: 'Switch-Dist-03',
    type: 'switch',
    host: '10.0.0.12',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 20,
    subnet: 'Core',
    latencyRange: [10, 65],
    latencyJitter: 10,
    uptimeProbability: 0.993,
    degradedProbability: 0.02,
    snmpProfile: {
      cpuRange: [12, 45],
      memoryRange: [35, 65],
      diskRange: [8, 18],
      interfaceCount: 48,
    },
  },

  // ═══ Access Switch ═════════════════════════════════════════
  {
    name: 'Switch-Access-01',
    type: 'switch',
    host: '10.0.20.1',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 30,
    subnet: 'Workstations',
    latencyRange: [20, 120],
    latencyJitter: 25,
    uptimeProbability: 0.985,
    degradedProbability: 0.04,
    snmpProfile: {
      cpuRange: [5, 25],
      memoryRange: [20, 45],
      diskRange: [3, 10],
      interfaceCount: 48,
    },
  },

  // ═══ Firewalls ══════════════════════════════════════════════
  {
    name: 'FW-Perimeter-01',
    type: 'firewall',
    host: '10.0.0.254',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 15,
    subnet: 'Core',
    latencyRange: [2, 8],
    latencyJitter: 1.8,
    uptimeProbability: 0.999,
    degradedProbability: 0.003,
  },
  {
    name: 'FW-DMZ-01',
    type: 'firewall',
    host: '172.16.0.254',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 15,
    subnet: 'DMZ',
    latencyRange: [2, 10],
    latencyJitter: 2.0,
    uptimeProbability: 0.998,
    degradedProbability: 0.005,
  },
  {
    name: 'FW-Internal-01',
    type: 'firewall',
    host: '10.0.0.253',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 20,
    subnet: 'Core',
    latencyRange: [1, 6],
    latencyJitter: 1.0,
    uptimeProbability: 0.999,
    degradedProbability: 0.002,
  },

  // ═══ Web Servers ════════════════════════════════════════════
  {
    name: 'WebSrv-Prod-01',
    type: 'server',
    host: '172.16.0.10',
    protocol: 'http',
    port: 443,
    intervalSeconds: 20,
    subnet: 'DMZ',
    latencyRange: [80, 200],
    latencyJitter: 35,
    uptimeProbability: 0.98,
    degradedProbability: 0.04,
    httpStatusWeights: { 200: 85, 301: 5, 503: 5, 0: 5 },
  },
  {
    name: 'WebSrv-Staging-01',
    type: 'server',
    host: '172.16.0.11',
    protocol: 'http',
    port: 8080,
    intervalSeconds: 30,
    subnet: 'DMZ',
    latencyRange: [100, 350],
    latencyJitter: 60,
    uptimeProbability: 0.92,
    degradedProbability: 0.06,
    httpStatusWeights: { 200: 75, 301: 5, 500: 10, 503: 5, 0: 5 },
  },

  // ═══ Database Servers ═══════════════════════════════════════
  {
    name: 'DB-Primary',
    type: 'server',
    host: '10.0.10.20',
    protocol: 'system',
    port: 5432,
    intervalSeconds: 15,
    subnet: 'Servers',
    latencyRange: [0, 0],
    latencyJitter: 0,
    uptimeProbability: 0.997,
    degradedProbability: 0.01,
    systemProfile: {
      cpuRange: [25, 75],
      memoryRange: [50, 85],
      diskRange: [40, 70],
    },
  },
  {
    name: 'DB-Replica',
    type: 'server',
    host: '10.0.10.21',
    protocol: 'system',
    port: 5432,
    intervalSeconds: 15,
    subnet: 'Servers',
    latencyRange: [0, 0],
    latencyJitter: 0,
    uptimeProbability: 0.995,
    degradedProbability: 0.015,
    systemProfile: {
      cpuRange: [15, 55],
      memoryRange: [40, 70],
      diskRange: [35, 65],
    },
  },

  // ═══ App Servers ════════════════════════════════════════════
  {
    name: 'AppSrv-01',
    type: 'server',
    host: '10.0.10.30',
    protocol: 'port',
    port: 8080,
    intervalSeconds: 20,
    subnet: 'Servers',
    latencyRange: [2, 25],
    latencyJitter: 5,
    uptimeProbability: 0.99,
    degradedProbability: 0.02,
    portList: [8080, 443],
  },
  {
    name: 'AppSrv-02',
    type: 'server',
    host: '10.0.10.31',
    protocol: 'port',
    port: 8080,
    intervalSeconds: 20,
    subnet: 'Servers',
    latencyRange: [3, 30],
    latencyJitter: 6,
    uptimeProbability: 0.96,
    degradedProbability: 0.03,
    portList: [8080, 443],
  },

  // ═══ File Server ════════════════════════════════════════════
  {
    name: 'FileServer-NAS',
    type: 'server',
    host: '10.0.10.50',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 30,
    subnet: 'Servers',
    latencyRange: [50, 200],
    latencyJitter: 40,
    uptimeProbability: 0.99,
    degradedProbability: 0.02,
    snmpProfile: {
      cpuRange: [5, 20],
      memoryRange: [30, 50],
      diskRange: [70, 92],
      interfaceCount: 4,
    },
  },

  // ═══ Mail Server ════════════════════════════════════════════
  {
    name: 'MailSrv-01',
    type: 'server',
    host: '172.16.0.25',
    protocol: 'port',
    port: 25,
    intervalSeconds: 30,
    subnet: 'DMZ',
    latencyRange: [5, 40],
    latencyJitter: 8,
    uptimeProbability: 0.98,
    degradedProbability: 0.02,
    portList: [25, 587],
  },

  // ═══ DNS Servers ════════════════════════════════════════════
  {
    name: 'DNS-Primary',
    type: 'server',
    host: '10.0.0.53',
    protocol: 'ping',
    port: 53,
    intervalSeconds: 10,
    subnet: 'Core',
    latencyRange: [0.5, 2],
    latencyJitter: 0.4,
    uptimeProbability: 0.9999,
    degradedProbability: 0.001,
  },
  {
    name: 'DNS-Secondary',
    type: 'server',
    host: '10.0.0.54',
    protocol: 'ping',
    port: 53,
    intervalSeconds: 10,
    subnet: 'Core',
    latencyRange: [0.5, 3],
    latencyJitter: 0.5,
    uptimeProbability: 0.999,
    degradedProbability: 0.002,
  },

  // ═══ PCs / Workstations ═════════════════════════════════════
  {
    name: 'PC-Admin-01',
    type: 'pc',
    host: '10.0.20.100',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 30,
    subnet: 'Workstations',
    latencyRange: [1, 8],
    latencyJitter: 2,
    uptimeProbability: 0.95,
    degradedProbability: 0.01,
    offlineHours: [0, 1, 2, 3, 4, 5, 6, 22, 23],
  },
  {
    name: 'PC-Admin-02',
    type: 'pc',
    host: '10.0.20.101',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 30,
    subnet: 'Workstations',
    latencyRange: [1, 10],
    latencyJitter: 2.5,
    uptimeProbability: 0.93,
    degradedProbability: 0.015,
    offlineHours: [0, 1, 2, 3, 4, 5, 6, 7, 21, 22, 23],
  },
  {
    name: 'PC-Dev-01',
    type: 'pc',
    host: '10.0.20.110',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 30,
    subnet: 'Workstations',
    latencyRange: [1, 6],
    latencyJitter: 1.5,
    uptimeProbability: 0.97,
    degradedProbability: 0.005,
    offlineHours: [0, 1, 2, 3, 4, 5, 23],
  },
  {
    name: 'PC-Dev-02',
    type: 'pc',
    host: '10.0.20.111',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 30,
    subnet: 'Workstations',
    latencyRange: [1, 7],
    latencyJitter: 1.8,
    uptimeProbability: 0.96,
    degradedProbability: 0.01,
    offlineHours: [0, 1, 2, 3, 4, 5, 6, 22, 23],
  },

  // ═══ Printers ═══════════════════════════════════════════════
  {
    name: 'Printer-Floor1',
    type: 'printer',
    host: '10.0.20.200',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 60,
    subnet: 'Workstations',
    latencyRange: [200, 500],
    latencyJitter: 80,
    uptimeProbability: 0.88,
    degradedProbability: 0.05,
    snmpProfile: {
      cpuRange: [2, 10],
      memoryRange: [20, 40],
      diskRange: [10, 30],
      interfaceCount: 2,
    },
  },
  {
    name: 'Printer-Floor2',
    type: 'printer',
    host: '10.0.20.201',
    protocol: 'snmp',
    port: 161,
    intervalSeconds: 60,
    subnet: 'Workstations',
    latencyRange: [180, 450],
    latencyJitter: 70,
    uptimeProbability: 0.90,
    degradedProbability: 0.04,
    snmpProfile: {
      cpuRange: [2, 8],
      memoryRange: [15, 35],
      diskRange: [8, 25],
      interfaceCount: 2,
    },
  },

  // ═══ Wireless Access Points ════════════════════════════════
  {
    name: 'AP-Lobby',
    type: 'access_point',
    host: '192.168.1.10',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 20,
    subnet: 'Management',
    latencyRange: [3, 15],
    latencyJitter: 4,
    uptimeProbability: 0.97,
    degradedProbability: 0.025,
  },
  {
    name: 'AP-Office',
    type: 'access_point',
    host: '192.168.1.11',
    protocol: 'ping',
    port: 0,
    intervalSeconds: 20,
    subnet: 'Management',
    latencyRange: [2, 12],
    latencyJitter: 3,
    uptimeProbability: 0.98,
    degradedProbability: 0.015,
  },
];
