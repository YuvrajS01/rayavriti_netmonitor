const ping = require('ping');

async function checkPing(device) {
  const start = Date.now();
  try {
    const res = await ping.promise.probe(device.host, { timeout: 5 });
    return {
      status: res.alive ? 'up' : 'down',
      responseTime: res.time === 'unknown' ? null : Number(res.time),
      value: res.alive ? 1 : 0,
      message: res.alive ? 'Ping reachable' : 'Ping unreachable',
      elapsedMs: Date.now() - start
    };
  } catch (error) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: `Ping error: ${error.message}`,
      elapsedMs: Date.now() - start
    };
  }
}

module.exports = { checkPing };
