const net = require('net');

const DEFAULT_PORTS = [
  21, 22, 23, 25, 53, 80, 110, 123, 135, 139, 143, 161, 389, 443, 445,
  465, 587, 636, 993, 995, 1433, 1521, 2049, 2375, 3000, 3306, 3389,
  5000, 5432, 5900, 6379, 8000, 8080, 8443, 9000, 9200, 9300, 11211, 27017
];

const SERVICE_BY_PORT = {
  21: 'FTP',
  22: 'SSH',
  23: 'Telnet',
  25: 'SMTP',
  53: 'DNS',
  80: 'HTTP',
  110: 'POP3',
  123: 'NTP',
  135: 'MS RPC',
  139: 'NetBIOS',
  143: 'IMAP',
  161: 'SNMP',
  389: 'LDAP',
  443: 'HTTPS',
  445: 'SMB',
  465: 'SMTPS',
  587: 'SMTP Submission',
  636: 'LDAPS',
  993: 'IMAPS',
  995: 'POP3S',
  1433: 'MSSQL',
  1521: 'Oracle',
  2049: 'NFS',
  2375: 'Docker API',
  3000: 'App Server',
  3306: 'MySQL',
  3389: 'RDP',
  5000: 'App Server',
  5432: 'PostgreSQL',
  5900: 'VNC',
  6379: 'Redis',
  8000: 'HTTP Alt',
  8080: 'HTTP Proxy',
  8443: 'HTTPS Alt',
  9000: 'App Server',
  9200: 'Elasticsearch',
  9300: 'Elasticsearch Transport',
  11211: 'Memcached',
  27017: 'MongoDB'
};

function normalizePorts(ports: any) {
  const list = Array.isArray(ports) && ports.length > 0 ? ports : DEFAULT_PORTS;
  return Array.from(new Set(
    list
      .map((p) => Number(p))
      .filter((p) => Number.isInteger(p) && p > 0 && p <= 65535)
  )).slice(0, 256);
}

function scanPort(host: string, port: number, timeoutMs = 1200): Promise<any> {
  return new Promise((resolve) => {
    const start = Date.now();
    const socket = new net.Socket();
    let completed = false;

    const finalize = (status, message = null) => {
      if (completed) {
        return;
      }
      completed = true;
      socket.destroy();
      resolve({
        port,
        status,
        serviceGuess: SERVICE_BY_PORT[port] || 'Unknown',
        responseTime: status === 'open' ? Date.now() - start : null,
        message
      });
    };

    socket.setTimeout(timeoutMs);
    socket.connect(port, host, () => finalize('open'));
    socket.on('timeout', () => finalize('closed', `Timeout after ${timeoutMs}ms`));
    socket.on('error', (error) => finalize('closed', error.message));
  });
}

async function scanPorts(host: string, options: any = {}) {
  const ports = normalizePorts(options.ports);
  const timeoutMs = Math.max(250, Math.min(5000, Number(options.timeoutMs || 1200)));
  const concurrency = Math.max(1, Math.min(32, Number(options.concurrency || 24)));
  const results = [];
  let index = 0;

  async function worker() {
    while (index < ports.length) {
      const port = ports[index];
      index += 1;
      results.push(await scanPort(host, port, timeoutMs));
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, ports.length) }, () => worker()));
  return results.sort((a, b) => a.port - b.port);
}

module.exports = {
  DEFAULT_PORTS,
  SERVICE_BY_PORT,
  scanPorts
};

export {};
