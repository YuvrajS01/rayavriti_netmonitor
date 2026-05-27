import fs from 'fs';
import path from 'path';
import logger from '../services/logger';

let Cap: any, decoders: any;
try {
  const capModule = require('cap');
  Cap = capModule.Cap;
  decoders = capModule.decoders;
} catch (err: any) {
  logger.warn({ err: err.message }, 'cap module not available');
  Cap = null;
  decoders = null;
}

import db from '../services/database';

const MAX_CAPTURE_DURATION_MS = 3 * 60 * 1000; // 3 minutes
const MAX_PACKET_BUFFER = 1000;
const PAYLOAD_PREVIEW_BYTES = 128;

const PROTOCOL_NAMES: Record<number, string> = {
  1: 'ICMP', 6: 'TCP', 17: 'UDP', 2: 'IGMP',
  47: 'GRE', 50: 'ESP', 58: 'ICMPv6', 89: 'OSPF'
};

// Active capture sessions: Map<sessionId, { cap, timer, packets[], stats }>
const activeSessions = new Map();

function listInterfaces() {
  const netDir = '/sys/class/net';
  const preferred = ['wlan0', 'eth0'];
  try {
    const all = fs.readdirSync(netDir).filter((name) => {
      // Skip virtual interfaces
      if (name === 'lo') return false;
      return true;
    });
    // Sort preferred interfaces first
    all.sort((a, b) => {
      const ai = preferred.indexOf(a);
      const bi = preferred.indexOf(b);
      if (ai !== -1 && bi !== -1) return ai - bi;
      if (ai !== -1) return -1;
      if (bi !== -1) return 1;
      return a.localeCompare(b);
    });

    return all.map((name) => {
      let addresses = [];
      try {
        const addrPath = path.join(netDir, name, 'address');
        const mac = fs.readFileSync(addrPath, 'utf-8').trim();
        if (mac && mac !== '00:00:00:00:00:00') addresses.push(mac);
      } catch (_e) { /* skip */ }

      let flags = [];
      try {
        const operstatePath = path.join(netDir, name, 'operstate');
        const state = fs.readFileSync(operstatePath, 'utf-8').trim();
        flags.push(state);
      } catch (_e) { /* skip */ }

      return { name, addresses, flags };
    });
  } catch (_e) {
    return [];
  }
}

function toHex(buffer: any, length: number) {
  const bytes = Math.min(length, PAYLOAD_PREVIEW_BYTES);
  let hex = '';
  for (let i = 0; i < bytes; i++) {
    hex += buffer[i].toString(16).padStart(2, '0');
    if (i < bytes - 1) hex += ' ';
  }
  return hex;
}

function toAscii(buffer: any, length: number) {
  const bytes = Math.min(length, PAYLOAD_PREVIEW_BYTES);
  let ascii = '';
  for (let i = 0; i < bytes; i++) {
    const ch = buffer[i];
    ascii += (ch >= 32 && ch <= 126) ? String.fromCharCode(ch) : '.';
  }
  return ascii;
}

function decodePacket(rawBuffer: any, nbytes: number) {
  if (!decoders) return null;

  try {
    // Decode Ethernet frame
    const ethInfo = decoders.Ethernet(rawBuffer);
    if (ethInfo.info.type !== decoders.PROTOCOL.ETHERNET.IPV4) {
      return {
        src_ip: ethInfo.info.srcmac || '?',
        dst_ip: ethInfo.info.dstmac || '?',
        src_port: 0,
        dst_port: 0,
        protocol: 'ETH',
        length: nbytes,
        payload_hex: toHex(rawBuffer, nbytes),
        info: `Ethernet type: 0x${ethInfo.info.type.toString(16)}`
      };
    }

    // Decode IPv4
    const ipInfo = decoders.IPV4(rawBuffer, ethInfo.offset);
    const proto = ipInfo.info.protocol;
    const protoName = PROTOCOL_NAMES[proto] || `PROTO_${proto}`;

    let srcPort = 0;
    let dstPort = 0;
    let info = `${protoName} ${ipInfo.info.srcaddr} → ${ipInfo.info.dstaddr}`;

    if (proto === 6) {
      // TCP
      try {
        const tcpInfo = decoders.TCP(rawBuffer, ipInfo.offset);
        srcPort = tcpInfo.info.srcport;
        dstPort = tcpInfo.info.dstport;
        const flags = [];
        if (tcpInfo.info.flags & 0x02) flags.push('SYN');
        if (tcpInfo.info.flags & 0x10) flags.push('ACK');
        if (tcpInfo.info.flags & 0x01) flags.push('FIN');
        if (tcpInfo.info.flags & 0x04) flags.push('RST');
        if (tcpInfo.info.flags & 0x08) flags.push('PSH');
        info = `TCP ${srcPort} → ${dstPort} [${flags.join(',')}] Len=${nbytes}`;
      } catch (_e) { /* truncated */ }
    } else if (proto === 17) {
      // UDP
      try {
        const udpInfo = decoders.UDP(rawBuffer, ipInfo.offset);
        srcPort = udpInfo.info.srcport;
        dstPort = udpInfo.info.dstport;
        info = `UDP ${srcPort} → ${dstPort} Len=${nbytes}`;
      } catch (_e) { /* truncated */ }
    } else if (proto === 1) {
      info = `ICMP ${ipInfo.info.srcaddr} → ${ipInfo.info.dstaddr}`;
    }

    return {
      src_ip: ipInfo.info.srcaddr,
      dst_ip: ipInfo.info.dstaddr,
      src_port: srcPort,
      dst_port: dstPort,
      protocol: protoName,
      length: nbytes,
      payload_hex: toHex(rawBuffer, nbytes),
      info
    };
  } catch (err: any) {
    return {
      src_ip: '?',
      dst_ip: '?',
      src_port: 0,
      dst_port: 0,
      protocol: 'UNKNOWN',
      length: nbytes,
      payload_hex: toHex(rawBuffer, nbytes),
      info: `Decode error: ${err.message}`
    };
  }
}

function startCapture(io: any, interfaceName: string, filter: string | null = null) {
  if (!Cap) {
    throw new Error('Packet capture not available (cap module not loaded). Ensure libpcap is installed.');
  }

  const result = db.createCaptureSession(interfaceName, filter as any);
  const sessionId = Number(result.lastInsertRowid);

  const cap = new Cap();
  const bufSize = 10 * 1024 * 1024; // 10MB buffer
  const snapLen = 65535;
  const rawBuffer = Buffer.alloc(snapLen);

  const sessionData: { cap: any; timer: any; packets: any[]; stats: { packetCount: number; bytesCaptured: number }; sessionId: number } = {
    cap,
    timer: null,
    packets: [],
    stats: { packetCount: 0, bytesCaptured: 0 },
    sessionId
  };

  try {
    // Verify interface exists in /sys/class/net before trying to open
    const ifacePath = `/sys/class/net/${interfaceName}`;
    if (!fs.existsSync(ifacePath)) {
      throw new Error(`Interface "${interfaceName}" not found on this system`);
    }

    // Pass interface name directly to libpcap (cap.open accepts device names)
    const linkType = cap.open(interfaceName, filter || '', bufSize, rawBuffer);
    cap.setMinBytes && cap.setMinBytes(0);

    let packetNo = 0;

    cap.on('packet', (nbytes: number) => {
      packetNo++;
      sessionData.stats.packetCount++;
      sessionData.stats.bytesCaptured += nbytes;

      const decoded = decodePacket(rawBuffer, nbytes);
      if (!decoded) return;

      const packet = {
        no: packetNo,
        session_id: sessionId,
        ...decoded,
        timestamp: new Date().toISOString()
      };

      // Keep in-memory buffer (circular)
      sessionData.packets.push(packet);
      if (sessionData.packets.length > MAX_PACKET_BUFFER) {
        sessionData.packets.shift();
      }

      // Emit to WebSocket subscribers
      if (io) {
        io.emit('packet:captured', packet);
      }
    });

    cap.on('error', (err: any) => {
      logger.error({ sessionId, err: err.message }, 'Packet capture error');
      stopCapture(io, sessionId, err.message);
    });

    // Auto-stop after max duration (3 minutes)
    sessionData.timer = setTimeout(() => {
      logger.info({ sessionId }, 'Capture session reached max duration, stopping');
      stopCapture(io, sessionId);
    }, MAX_CAPTURE_DURATION_MS);

    activeSessions.set(sessionId, sessionData);

    if (io) {
      io.emit('capture:status', {
        sessionId,
        status: 'running',
        interface: interfaceName,
        filter
      });
    }

    logger.info({ sessionId, interface: interfaceName, filter }, 'Capture session started');
    return sessionId;
  } catch (err: any) {
    db.stopCaptureSession(sessionId, 0, 0, err.message);
    try { cap.close(); } catch (_e) { /* ignore */ }
    throw err;
  }
}

function stopCapture(io: any, sessionId: number, errorMessage: string | null = null) {
  const session = activeSessions.get(sessionId);
  if (!session) return false;

  if (session.timer) {
    clearTimeout(session.timer);
    session.timer = null;
  }

  try {
    session.cap.close();
  } catch (_e) { /* ignore */ }

  db.stopCaptureSession(
    sessionId,
    session.stats.packetCount,
    session.stats.bytesCaptured,
    errorMessage as any
  );

  if (io) {
    io.emit('capture:status', {
      sessionId,
      status: errorMessage ? 'error' : 'stopped',
      packetCount: session.stats.packetCount,
      bytesCaptured: session.stats.bytesCaptured,
      error: errorMessage || null
    });
  }

  activeSessions.delete(sessionId);
  logger.info({ sessionId, packetCount: session.stats.packetCount, bytes: session.stats.bytesCaptured }, 'Capture session stopped');
  return true;
}

function getSessionPackets(sessionId: number, { limit = 200, offset = 0 } = {}) {
  const session = activeSessions.get(sessionId);
  if (!session) return [];
  const packets = session.packets;
  return packets.slice(offset, offset + limit);
}

function getSessionStats(sessionId: number) {
  const session = activeSessions.get(sessionId);
  if (!session) return null;
  return { ...session.stats };
}

function stopAllCaptures(io: any) {
  for (const [id] of activeSessions) {
    stopCapture(io, id);
  }
}

export {
  listInterfaces,
  startCapture,
  stopCapture,
  getSessionPackets,
  getSessionStats,
  stopAllCaptures
};
