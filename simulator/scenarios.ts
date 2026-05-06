// ─── Scripted Failure/Recovery Scenarios ──────────────────────
// Timeline-driven events that exercise alert generation, status
// transitions, and anomaly detection in the monitoring system.

export type StatusOverride = 'up' | 'down' | 'warning' | undefined;

export interface ScenarioEvent {
  /** Seconds after simulation start when this event triggers */
  triggerAtSec: number;
  /** Seconds after simulation start when this event ends (device reverts to normal) */
  endAtSec: number;
  /** Which device(s) are affected (by name) */
  deviceNames: string[];
  /** Override status during the event window */
  statusOverride: StatusOverride;
  /** Whether to spike NetFlow traffic (multiplier) */
  flowTrafficMultiplier?: number;
  /** Human-readable description for CLI logging */
  description: string;
}

// ─── Scenario Presets ───────────────────────────────────────

/**
 * "mixed" — The default. A realistic sequence of outages, degradation,
 * and spikes that exercises every alerting path.
 */
const MIXED_SCENARIOS: ScenarioEvent[] = [
  // 1. Link Flap — access switch oscillates
  {
    triggerAtSec: 60,
    endAtSec: 120,
    deviceNames: ['Switch-Access-01'],
    statusOverride: 'warning',
    description: 'Link Flap: Switch-Access-01 port oscillation',
  },

  // 2. Server Crash — AppSrv-02 goes fully down
  {
    triggerAtSec: 180,
    endAtSec: 360,
    deviceNames: ['AppSrv-02'],
    statusOverride: 'down',
    description: 'Server Crash: AppSrv-02 completely offline',
  },

  // 3. DDoS simulation — traffic spike to web server
  {
    triggerAtSec: 240,
    endAtSec: 420,
    deviceNames: ['WebSrv-Prod-01'],
    statusOverride: 'warning',
    flowTrafficMultiplier: 10,
    description: 'DDoS Simulation: 10x traffic spike on WebSrv-Prod-01',
  },

  // 4. Disk Full — NAS runs out of space
  {
    triggerAtSec: 300,
    endAtSec: 480,
    deviceNames: ['FileServer-NAS'],
    statusOverride: 'warning',
    description: 'Disk Full: FileServer-NAS disk usage hits 98%',
  },

  // 5. DNS Outage — secondary DNS down
  {
    triggerAtSec: 480,
    endAtSec: 540,
    deviceNames: ['DNS-Secondary'],
    statusOverride: 'down',
    description: 'DNS Outage: DNS-Secondary unreachable',
  },

  // 6. DNS Primary degradation (latency spike when secondary is down)
  {
    triggerAtSec: 480,
    endAtSec: 540,
    deviceNames: ['DNS-Primary'],
    statusOverride: 'warning',
    description: 'DNS Degradation: DNS-Primary latency spike (compensating for secondary outage)',
  },

  // 7. Gradual degradation — DB primary slows down over time
  // This is handled specially in run.ts via incremental latency override
  {
    triggerAtSec: 30,
    endAtSec: 540,
    deviceNames: ['DB-Primary'],
    statusOverride: undefined, // managed by gradual degradation logic
    description: 'Gradual Degradation: DB-Primary response time slowly climbing',
  },

  // 8. Printer offline — intermittent
  {
    triggerAtSec: 150,
    endAtSec: 240,
    deviceNames: ['Printer-Floor1'],
    statusOverride: 'down',
    description: 'Printer Offline: Printer-Floor1 unreachable (paper jam?)',
  },

  // 9. AP drops — wireless interference
  {
    triggerAtSec: 360,
    endAtSec: 420,
    deviceNames: ['AP-Lobby'],
    statusOverride: 'down',
    description: 'Wireless Drop: AP-Lobby lost connectivity',
  },

  // 10. Full recovery — everything returns to normal
  // (this happens naturally when events end)
];

/**
 * "stable" — All devices stay healthy. Good for baseline testing.
 */
const STABLE_SCENARIOS: ScenarioEvent[] = [];

/**
 * "degraded" — Multiple devices in warning state simultaneously.
 */
const DEGRADED_SCENARIOS: ScenarioEvent[] = [
  {
    triggerAtSec: 30,
    endAtSec: 570,
    deviceNames: ['WebSrv-Staging-01', 'AppSrv-02', 'Printer-Floor1', 'Printer-Floor2'],
    statusOverride: 'warning',
    description: 'Sustained Degradation: multiple devices showing warning signs',
  },
  {
    triggerAtSec: 120,
    endAtSec: 570,
    deviceNames: ['Switch-Access-01', 'AP-Lobby'],
    statusOverride: 'warning',
    description: 'Network Degradation: access layer performance issues',
  },
];

/**
 * "outage" — Catastrophic: most devices go down.
 * Tests the dashboard's ability to show a dire situation.
 */
const OUTAGE_SCENARIOS: ScenarioEvent[] = [
  {
    triggerAtSec: 60,
    endAtSec: 300,
    deviceNames: [
      'Router-Core-01', 'Router-Core-02',
      'Switch-Dist-01', 'Switch-Dist-02', 'Switch-Dist-03',
      'Switch-Access-01',
      'AppSrv-01', 'AppSrv-02',
      'WebSrv-Prod-01', 'WebSrv-Staging-01',
      'DB-Primary', 'DB-Replica',
    ],
    statusOverride: 'down',
    description: 'Major Outage: core infrastructure failure — 12 devices down',
  },
  {
    triggerAtSec: 300,
    endAtSec: 360,
    deviceNames: [
      'Router-Core-01', 'Switch-Dist-01', 'Switch-Dist-02',
      'AppSrv-01', 'WebSrv-Prod-01', 'DB-Primary',
    ],
    statusOverride: 'warning',
    description: 'Partial Recovery: core devices coming back online (degraded)',
  },
  // After T+360, everything reverts to normal (full recovery)
];

// ─── Scenario Manager ───────────────────────────────────────

const PRESETS: Record<string, ScenarioEvent[]> = {
  mixed: MIXED_SCENARIOS,
  stable: STABLE_SCENARIOS,
  degraded: DEGRADED_SCENARIOS,
  outage: OUTAGE_SCENARIOS,
};

export class ScenarioEngine {
  private events: ScenarioEvent[];
  private activeEvents: Set<ScenarioEvent> = new Set();
  private triggeredLog: string[] = [];

  constructor(preset: string = 'mixed') {
    this.events = [...(PRESETS[preset] || PRESETS.mixed)];
    // Sort by trigger time
    this.events.sort((a, b) => a.triggerAtSec - b.triggerAtSec);
  }

  /**
   * Call this every tick. Returns which events just triggered or ended.
   */
  tick(elapsedSec: number): { triggered: ScenarioEvent[]; ended: ScenarioEvent[] } {
    const triggered: ScenarioEvent[] = [];
    const ended: ScenarioEvent[] = [];

    for (const event of this.events) {
      const wasActive = this.activeEvents.has(event);

      if (elapsedSec >= event.triggerAtSec && elapsedSec < event.endAtSec) {
        if (!wasActive) {
          this.activeEvents.add(event);
          triggered.push(event);
          this.triggeredLog.push(
            `[T+${event.triggerAtSec}s] ${event.description}`
          );
        }
      } else if (wasActive && elapsedSec >= event.endAtSec) {
        this.activeEvents.delete(event);
        ended.push(event);
        this.triggeredLog.push(
          `[T+${event.endAtSec}s] RECOVERED: ${event.description}`
        );
      }
    }

    return { triggered, ended };
  }

  /**
   * Get the current status override for a given device name.
   * Returns undefined if no scenario is affecting this device.
   */
  getDeviceOverride(deviceName: string): StatusOverride {
    for (const event of this.activeEvents) {
      if (event.deviceNames.includes(deviceName) && event.statusOverride) {
        return event.statusOverride;
      }
    }
    return undefined;
  }

  /**
   * Get the current flow traffic multiplier.
   * Returns 1 if no traffic-affecting scenario is active.
   */
  getFlowMultiplier(): number {
    let max = 1;
    for (const event of this.activeEvents) {
      if (event.flowTrafficMultiplier && event.flowTrafficMultiplier > max) {
        max = event.flowTrafficMultiplier;
      }
    }
    return max;
  }

  /**
   * Check if the gradual degradation scenario for DB-Primary is active.
   * Returns a 0-1 factor (0 = start of degradation, 1 = peak) for interpolation.
   */
  getGradualDegradationFactor(deviceName: string, elapsedSec: number): number | null {
    for (const event of this.activeEvents) {
      if (
        event.deviceNames.includes(deviceName) &&
        event.statusOverride === undefined &&
        event.description.includes('Gradual')
      ) {
        const duration = event.endAtSec - event.triggerAtSec;
        const elapsed = elapsedSec - event.triggerAtSec;
        return Math.min(1, elapsed / duration);
      }
    }
    return null;
  }

  /** All log entries produced so far */
  getLog(): string[] {
    return [...this.triggeredLog];
  }
}
