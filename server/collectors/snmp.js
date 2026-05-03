const snmp = require('net-snmp');

const OIDS = {
  sysUpTime: '1.3.6.1.2.1.1.3.0',
  hrProcessorLoad: '1.3.6.1.2.1.25.3.3.1.2',
  hrStorageTable: '1.3.6.1.2.1.25.2.3.1',
  hrStorageRam: '1.3.6.1.2.1.25.2.1.2',
  hrStorageFixedDisk: '1.3.6.1.2.1.25.2.1.4'
};

function resolveSnmpVersion(value) {
  const raw = String(value || '2c').toLowerCase();
  if (raw === '1' || raw === 'v1') {
    return snmp.Version1;
  }
  if (raw === '2' || raw === '2c' || raw === 'v2' || raw === 'v2c') {
    return snmp.Version2c;
  }
  return null;
}

function toNumber(value) {
  if (value === null || typeof value === 'undefined') {
    return null;
  }
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : null;
  }
  if (Buffer.isBuffer(value)) {
    const parsed = Number(value.toString('utf-8'));
    return Number.isFinite(parsed) ? parsed : null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function safePercent(used, total) {
  if (!total) {
    return 0;
  }
  return Math.round((used / total) * 1000) / 10;
}

function toGb(bytes) {
  return Math.round((bytes / (1024 * 1024 * 1024)) * 10) / 10;
}

function getScalar(session, oid) {
  return new Promise((resolve, reject) => {
    session.get([oid], (error, varbinds) => {
      if (error) {
        return reject(error);
      }
      const [vb] = varbinds || [];
      if (!vb) {
        return resolve(null);
      }
      if (snmp.isVarbindError(vb)) {
        return reject(new Error(snmp.varbindError(vb)));
      }
      return resolve(vb.value);
    });
  });
}

function collectSubtree(session, oid, maxRepetitions = 20) {
  return new Promise((resolve, reject) => {
    const results = [];
    session.subtree(
      oid,
      maxRepetitions,
      (varbinds) => {
        for (const vb of varbinds) {
          if (snmp.isVarbindError(vb)) {
            continue;
          }
          results.push(vb);
        }
      },
      (error) => {
        if (error) {
          return reject(error);
        }
        return resolve(results);
      }
    );
  });
}

function collectTableColumns(session, oid, columns, maxRepetitions = 20) {
  return new Promise((resolve, reject) => {
    session.tableColumns(oid, columns, maxRepetitions, (error, table) => {
      if (error) {
        return reject(error);
      }
      return resolve(table || {});
    });
  });
}

function sumStorage(table, typeOid) {
  let totalBytes = 0;
  let usedBytes = 0;
  for (const row of Object.values(table)) {
    if (!row) {
      continue;
    }
    const rowType = String(row[2] ?? '');
    if (rowType !== typeOid) {
      continue;
    }
    const units = toNumber(row[4]);
    const size = toNumber(row[5]);
    const used = toNumber(row[6]);
    if (!units || !size) {
      continue;
    }
    const total = units * size;
    const usedTotal = units * (used || 0);
    totalBytes += total;
    usedBytes += usedTotal;
  }

  return {
    totalBytes,
    usedBytes
  };
}

async function checkSnmp(device) {
  const start = Date.now();
  const version = resolveSnmpVersion(device.snmp_version || device.snmpVersion);
  if (!version) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: 'SNMP version not supported (use v1 or v2c)'
    };
  }

  const community = device.snmp_community || device.snmpCommunity || 'public';
  const session = snmp.createSession(String(device.host), community, {
    port: Number(device.port || 161),
    retries: 1,
    timeout: 5000,
    version
  });

  try {
    const [uptimeValue, cpuVarbinds, storageTable] = await Promise.all([
      getScalar(session, OIDS.sysUpTime),
      collectSubtree(session, OIDS.hrProcessorLoad),
      collectTableColumns(session, OIDS.hrStorageTable, [2, 3, 4, 5, 6])
    ]);

    const cpuLoads = cpuVarbinds
      .map((vb) => toNumber(vb.value))
      .filter((val) => typeof val === 'number');
    const cpuAvg = cpuLoads.length
      ? Math.round((cpuLoads.reduce((sum, val) => sum + val, 0) / cpuLoads.length) * 10) / 10
      : 0;

    const memoryTotals = sumStorage(storageTable, OIDS.hrStorageRam);
    const diskTotals = sumStorage(storageTable, OIDS.hrStorageFixedDisk);

    const memoryPercent = safePercent(memoryTotals.usedBytes, memoryTotals.totalBytes);
    const diskPercent = safePercent(diskTotals.usedBytes, diskTotals.totalBytes);

    const uptimeTicks = toNumber(uptimeValue);
    const uptimeSeconds = uptimeTicks ? Math.round(uptimeTicks / 100) : 0;

    const resourceInfo = {
      cpu: {
        usage: cpuAvg,
        cores: cpuLoads.length
      },
      memory: {
        used: toGb(memoryTotals.usedBytes),
        total: toGb(memoryTotals.totalBytes),
        percent: memoryPercent
      },
      disk: {
        used: toGb(diskTotals.usedBytes),
        total: toGb(diskTotals.totalBytes),
        percent: diskPercent
      },
      uptime: uptimeSeconds
    };

    let status = 'up';
    if (cpuAvg > 90 || memoryPercent > 95 || diskPercent > 95) {
      status = 'warning';
    }

    return {
      status,
      responseTime: Date.now() - start,
      value: memoryPercent || cpuAvg || 0,
      message: JSON.stringify(resourceInfo)
    };
  } catch (error) {
    return {
      status: 'down',
      responseTime: null,
      value: 0,
      message: `SNMP error: ${error.message}`
    };
  } finally {
    session.close();
  }
}

module.exports = { checkSnmp };
