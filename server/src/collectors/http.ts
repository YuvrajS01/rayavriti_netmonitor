async function checkHttp(device: any) {
  const start = Date.now();
  try {
    const url = device.host.startsWith('http://') || device.host.startsWith('https://')
      ? device.host
      : `http://${device.host}`;

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 8000);

    const response = await fetch(url, { signal: controller.signal, method: 'GET' });
    clearTimeout(timeout);

    const responseTime = Date.now() - start;

    return {
      status: response.ok ? 'up' : 'degraded',
      responseTime,
      value: response.status,
      message: `HTTP ${response.status}`
    };
  } catch (error: any) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: `HTTP error: ${error.message}`
    };
  }
}

export { checkHttp };
