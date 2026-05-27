import db from './database';

/**
 * Formats byte counts into human-readable strings.
 */
function formatBytes(bytes: any) {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

/**
 * Get top talkers with percentage calculations.
 */
function getTopTalkersWithPercent({ from, to, limit = 10, direction = 'src' }: any = {}) {
  const rows: any[] = db.getTopTalkers({ from, to, limit, direction });
  const totalBytes: number = rows.reduce((sum: any, r: any) => sum + Number(r.bytes || 0), 0);

  return rows.map((r: any) => ({
    ip: r.ip,
    bytes: Number(r.bytes || 0),
    bytesFormatted: formatBytes(Number(r.bytes || 0)),
    packets: Number(r.packets || 0),
    flows: Number(r.flows || 0),
    percentage: totalBytes > 0
      ? Number(((Number(r.bytes || 0) / totalBytes) * 100).toFixed(1))
      : 0
  }));
}

/**
 * Get protocol distribution with percentage calculations.
 */
function getProtocolBreakdown({ from, to }: any = {}) {
  const rows: any[] = db.getProtocolDistribution({ from, to });
  const totalBytes: number = rows.reduce((sum: any, r: any) => sum + Number(r.bytes || 0), 0);

  return rows.map((r: any) => ({
    protocol_name: r.protocol_name || `Protocol ${r.protocol}`,
    protocol_number: r.protocol,
    bytes: Number(r.bytes || 0),
    bytesFormatted: formatBytes(Number(r.bytes || 0)),
    packets: Number(r.packets || 0),
    flows: Number(r.flows || 0),
    percentage: totalBytes > 0
      ? Number(((Number(r.bytes || 0) / totalBytes) * 100).toFixed(1))
      : 0
  }));
}

/**
 * Get comprehensive flow stats with formatted values.
 */
function getFlowSummary() {
  const stats = db.getFlowStats();
  return {
    totalFlows: Number(stats.total_flows || 0),
    totalBytes: Number(stats.total_bytes || 0),
    totalBytesFormatted: formatBytes(Number(stats.total_bytes || 0)),
    totalPackets: Number(stats.total_packets || 0),
    uniqueSources: Number(stats.unique_sources || 0),
    uniqueDestinations: Number(stats.unique_destinations || 0),
    activeCollectors: stats.activeCollectors || 0,
    collectorTypes: stats.collectorTypes || []
  };
}

/**
 * Detect potential traffic anomalies by comparing recent traffic to baseline.
 */
function detectAnomalies() {
  const now = new Date();
  const recent = new Date(now.getTime() - 5 * 60 * 1000).toISOString().slice(0, 19).replace('T', ' ');
  const baseline = new Date(now.getTime() - 60 * 60 * 1000).toISOString().slice(0, 19).replace('T', ' ');

  const recentStats: any[] = db.getFlowRecords({ from: recent, limit: 10000 });
  const baselineStats: any[] = db.getFlowRecords({ from: baseline, to: recent, limit: 10000 });

  const anomalies: any[] = [];

  if (baselineStats.length > 0) {
    const recentBytes: number = recentStats.reduce((s: any, r: any) => s + (r.bytes || 0), 0);
    const baselineBytes: number = baselineStats.reduce((s: any, r: any) => s + (r.bytes || 0), 0);
    const baselineAvg5min = (baselineBytes / 12); // ~55 minute window / 11 five-min blocks

    if (baselineAvg5min > 0 && recentBytes > baselineAvg5min * 3) {
      anomalies.push({
        type: 'traffic_spike',
        severity: 'warning',
        message: `Traffic spike detected: ${formatBytes(recentBytes)} in last 5 min vs ${formatBytes(Math.round(baselineAvg5min))} avg`,
        timestamp: now.toISOString()
      });
    }
  }

  // Check for unusual high-port traffic
  const unusualPorts = recentStats.filter(
    (r: any) => (r.dst_port > 10000 && r.bytes > 1024 * 1024) // >1MB to high ports
  );
  if (unusualPorts.length > 5) {
    anomalies.push({
      type: 'unusual_ports',
      severity: 'info',
      message: `${unusualPorts.length} flows to unusual high ports detected`,
      timestamp: now.toISOString()
    });
  }

  return anomalies;
}

export {
  formatBytes,
  getTopTalkersWithPercent,
  getProtocolBreakdown,
  getFlowSummary,
  detectAnomalies
};
