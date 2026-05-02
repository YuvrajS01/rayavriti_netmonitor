const os = require('os');
const { execSync } = require('child_process');

function getCpuUsage() {
  const cpus = os.cpus();
  let totalIdle = 0;
  let totalTick = 0;
  for (const cpu of cpus) {
    for (const type in cpu.times) {
      totalTick += cpu.times[type];
    }
    totalIdle += cpu.times.idle;
  }
  const idle = totalIdle / cpus.length;
  const total = totalTick / cpus.length;
  const usage = ((total - idle) / total) * 100;
  return {
    usage: Math.round(usage * 10) / 10,
    cores: cpus.length,
    model: cpus[0]?.model || 'Unknown',
  };
}

function getDiskUsage() {
  try {
    const output = execSync("df -B1 / | tail -1", { encoding: 'utf-8' });
    const parts = output.trim().split(/\s+/);
    const total = parseInt(parts[1], 10);
    const used = parseInt(parts[2], 10);
    return {
      used: Math.round(used / (1024 * 1024 * 1024) * 10) / 10,
      total: Math.round(total / (1024 * 1024 * 1024) * 10) / 10,
      percent: Math.round((used / total) * 100 * 10) / 10,
    };
  } catch {
    return { used: 0, total: 0, percent: 0 };
  }
}

async function collect() {
  const totalMem = os.totalmem();
  const freeMem = os.freemem();
  const usedMem = totalMem - freeMem;
  const memPercent = Math.round((usedMem / totalMem) * 100 * 10) / 10;

  const cpu = getCpuUsage();
  const disk = getDiskUsage();
  const loadAvg = os.loadavg().map((v) => Math.round(v * 100) / 100);
  const uptime = Math.round(os.uptime());

  const systemInfo = {
    cpu,
    memory: {
      used: Math.round(usedMem / (1024 * 1024 * 1024) * 10) / 10,
      total: Math.round(totalMem / (1024 * 1024 * 1024) * 10) / 10,
      percent: memPercent,
    },
    disk,
    uptime,
    loadAvg,
  };

  // Determine status based on thresholds
  let status = 'ok';
  if (cpu.usage > 90 || memPercent > 95) {
    status = 'warning';
  }

  return {
    status,
    responseTime: 0,
    value: memPercent,
    message: JSON.stringify(systemInfo),
  };
}

module.exports = { collect };
