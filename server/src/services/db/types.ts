/** Repository interfaces for the data layer. */

export interface IDeviceRepo {
  getDevices(): any[];
  getDevice(id: any): any;
  addDevice(device: any): any;
  updateDevice(id: any, device: any): any;
  deleteDevice(id: any): any;
}

export interface ISensorRepo {
  listSensors(opts?: any): any[];
  getSensor(id: any): any;
  getSensorsByDevice(deviceId: any): any[];
  addSensor(sensor: any): any;
  updateSensor(id: any, sensor: any): any;
  deleteSensor(id: any): any;
  getPrimarySensorForDevice(deviceId: any): any;
}

export interface IMetricRepo {
  recordMetric(deviceId: any, status: any, responseTime: any, value: any, message: any, sensorId?: any): any;
  getDeviceMetrics(deviceId: any, limit?: number): any[];
  queryMetrics(params: { deviceId: number; from: string; to: string }): any[];
  getRecentMetrics(opts?: any): any[];
  getLatestMetrics(): any[];
  getMetricsForReport(params: any): any[];
  getReportTimeseries(params: any): any[];
  getMetricsForTrend(windowHours?: number): { recent: any[]; baseline: any[] };
  getMetricsInWindow(fromIso: string, toIso: string): any[];
  getDeviceBreakdownForReport(params: any): any[];
}

export interface IAlertRepo {
  getActiveAlerts(): any[];
  getAlertById(id: any): any;
  getAlerts(opts?: any): any[];
  getAlertCounts(): any;
  createAlert(deviceId: any, severity: any, message: any, status?: string, comment?: any): any;
  findActiveAlert(deviceId: any, message: any): any;
  updateAlert(id: any, patch: any): any;
  deleteAlert(id: any): any;
  acknowledgeAlert(id: any, comment?: any): any;
  resolveAlert(id: any): any;
  getAlertsForReport(params: any): any[];
}

export interface IFlowRepo {
  insertFlowRecord(record: any): any;
  insertFlowBatch(records: any[]): void;
  getFlowRecords(opts?: any): any[];
  getTopTalkers(opts?: any): any[];
  getProtocolDistribution(opts?: any): any[];
  getFlowTimeseries(opts?: any): any[];
  getFlowStats(): any;
}

export interface IPortScanRepo {
  upsertPortScanResult(data: any): any;
  getPortScanResults(deviceId: any): any[];
}

export interface ICaptureRepo {
  createCaptureSession(interfaceName: string, filter?: string | null): any;
  stopCaptureSession(id: any, packetCount?: number, bytesCaptured?: number, errorMessage?: string | null): any;
  getCaptureSession(id: any): any;
  getCaptureSessions(limit?: number): any[];
  updateCaptureSessionStats(id: any, packetCount: number, bytesCaptured: number): any;
}

export interface IDashboardRepo {
  listDashboards(): any[];
  getDashboard(id: any): any;
  createDashboard(data: any): any;
  updateDashboard(id: any, data: any): any;
  deleteDashboard(id: any): any;
}

export interface IDatabase extends
  IDeviceRepo,
  ISensorRepo,
  IMetricRepo,
  IAlertRepo,
  IFlowRepo,
  IPortScanRepo,
  ICaptureRepo,
  IDashboardRepo {
  /** Execute a raw SQL statement (for health checks, retention, etc.) */
  raw(sql: string, ...params: any[]): any;
  getStats(): any;
  getUserByUsername(username: any): any;
  verifyApiKey(rawKey: any): any;
}
