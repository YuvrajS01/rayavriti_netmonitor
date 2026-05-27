import net from 'net';

function checkPort(device: any) {
  return new Promise((resolve) => {
    const start = Date.now();
    const timeoutMs = 5000;

    const socket = new net.Socket();
    let completed = false;

    const finalize = (result: any) => {
      if (completed) {
        return;
      }
      completed = true;
      socket.destroy();
      resolve(result);
    };

    socket.setTimeout(timeoutMs);

    socket.connect(Number(device.port || 80), device.host, () => {
      finalize({
        status: 'up',
        responseTime: Date.now() - start,
        value: 1,
        message: `Port ${device.port} open`
      });
    });

    socket.on('error', (error: any) => {
      finalize({
        status: 'down',
        responseTime: null,
        value: 0,
        message: `Port check failed: ${error.message}`
      });
    });

    socket.on('timeout', () => {
      finalize({
        status: 'down',
        responseTime: null,
        value: 0,
        message: `Port check timeout after ${timeoutMs}ms`
      });
    });
  });
}

export { checkPort };
