import Collector from 'node-netflowv9';
import db from '../services/database';
import logger from '../services/logger';

const PROTOCOL_MAP: Record<number, string> = {
  1: 'ICMP', 2: 'IGMP', 6: 'TCP', 17: 'UDP', 47: 'GRE',
  50: 'ESP', 51: 'AH', 58: 'ICMPv6', 89: 'OSPF', 132: 'SCTP'
};

function protoName(num: number) {
  return PROTOCOL_MAP[num] || `PROTO_${num}`;
}

const buffer: any[] = [];
let flushTimer: ReturnType<typeof setInterval> | null = null;

function flushBuffer(io: any) {
  if (buffer.length === 0) return;
  const batch = buffer.splice(0, buffer.length);
  try {
    db.insertFlowBatch(batch);
  } catch (err: any) {
    logger.error({ err: err.message }, 'NetFlow DB insert error');
  }

  if (io) {
    const summary = {
      count: batch.length,
      totalBytes: batch.reduce((s, r) => s + (r.bytes || 0), 0),
      totalPackets: batch.reduce((s, r) => s + (r.packets || 0), 0),
      timestamp: new Date().toISOString(),
      protocols: [...new Set(batch.map((r) => r.protocol_name).filter(Boolean))],
      sample: batch.slice(0, 5).map((r) => ({
        src: `${r.src_ip}:${r.src_port || '*'}`,
        dst: `${r.dst_ip}:${r.dst_port || '*'}`,
        proto: r.protocol_name,
        bytes: r.bytes
      }))
    };
    io.emit('flow:update', summary);
  }
}

function normalizeNetflowRecord(flow: any, header: any, rinfo: any) {
  const collectorType = header.version === 5
    ? 'netflow_v5'
    : header.version === 9
      ? 'netflow_v9'
      : header.version === 10
        ? 'ipfix'
        : `netflow_v${header.version}`;

  const srcIp = flow.ipv4_src_addr || flow.src || flow.srcaddr || '0.0.0.0';
  const dstIp = flow.ipv4_dst_addr || flow.dst || flow.dstaddr || '0.0.0.0';
  const proto = Number(flow.protocol || flow.prot || 0);

  return {
    collector_type: collectorType,
    src_ip: srcIp,
    dst_ip: dstIp,
    src_port: Number(flow.l4_src_port || flow.srcport || 0),
    dst_port: Number(flow.l4_dst_port || flow.dstport || 0),
    protocol: proto,
    protocol_name: protoName(proto),
    bytes: Number(flow.in_bytes || flow.octetDeltaCount || flow.dOctets || 0),
    packets: Number(flow.in_pkts || flow.packetDeltaCount || flow.dPkts || 0),
    flow_start: flow.first_switched ? new Date(flow.first_switched).toISOString() : null,
    flow_end: flow.last_switched ? new Date(flow.last_switched).toISOString() : null,
    input_interface: Number(flow.input_snmp || flow.input || 0) || null,
    output_interface: Number(flow.output_snmp || flow.output || 0) || null,
    tcp_flags: Number(flow.tcp_flags || 0) || null,
    tos: Number(flow.src_tos || flow.tos || 0) || null,
    src_as: Number(flow.src_as || 0) || null,
    dst_as: Number(flow.dst_as || 0) || null,
    exporter_ip: rinfo?.address || null
  };
}

let netflowCollector: any = null;

function startNetflowCollector(io: any, port = 2055) {
  try {
    netflowCollector = Collector({ port });

    netflowCollector.on('data', (data: any) => {
      const header = data.header || {};
      const rinfo = data.rinfo || {};
      const flows = data.flow || [];

      for (const flow of flows) {
        try {
          const record = normalizeNetflowRecord(flow, header, rinfo);
          buffer.push(record);
        } catch (err: any) {
          logger.error({ err: err.message }, 'NetFlow parse error');
        }
      }

      if (buffer.length >= 100) {
        flushBuffer(io);
      }
    });

    netflowCollector.on('error', (err: any) => {
      logger.error({ port, err: err.message }, 'NetFlow collector error');
    });

    // Flush buffer every 2 seconds
    flushTimer = setInterval(() => flushBuffer(io), 2000);

    logger.info({ port }, 'NetFlow collector listening');
  } catch (err: any) {
    logger.error({ port, err: err.message }, 'Failed to start NetFlow collector');
  }
}

function stopNetflowCollector() {
  if (flushTimer) {
    clearInterval(flushTimer);
    flushTimer = null;
  }
  if (netflowCollector) {
    try {
      netflowCollector.close();
    } catch (_e) { /* ignore */ }
    netflowCollector = null;
  }
}

export { startNetflowCollector, stopNetflowCollector };
