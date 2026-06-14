import type { DashboardSnapshot } from './dashboard-snapshot.types';

/**
 * EMPTY_DASHBOARD_SNAPSHOT provides a safe passive-only bootstrap state before backend events arrive.
 */
export const EMPTY_DASHBOARD_SNAPSHOT: DashboardSnapshot = {
  publishedAt: '',
  confirmed: {
    ollama: {
      status: 'unavailable',
      reachable: false,
      version: '',
      primaryModel: '',
      runningModels: [],
      catalogModelCount: 0,
      observedAt: '',
      lastConfirmedAt: '',
    },
    system: {
      observedAt: '',
      process: {
        status: 'unavailable',
        found: false,
        pid: 0,
        cpuPercent: 0,
        rssBytes: 0,
      },
      connections: {
        status: 'unavailable',
        count: 0,
      },
      host: {
        status: 'unavailable',
        cpuPercent: 0,
        memoryUsedBytes: 0,
        memoryTotalBytes: 0,
      },
    },
  },
  inferred: {
    current: {
      kind: 'inferred-unknown',
      truth: 'inferred',
      model: '',
      confidence: 'low',
      observedAt: '',
      evidence: [],
    },
    recent: [],
  },
  recent: {
    confirmedModels: [],
  },
  health: {
    ollama: { status: 'unavailable', healthy: false, supported: true, observedAt: '', error: '' },
    process: { status: 'unavailable', healthy: false, supported: true, observedAt: '', error: '' },
    connections: { status: 'unsupported', healthy: false, supported: false, observedAt: '', error: '' },
    host: { status: 'unavailable', healthy: false, supported: true, observedAt: '', error: '' },
  },
  passive: {
    mode: 'passive-only',
    exactRequestLatencyAvailable: false,
    exactTokenCountsAvailable: false,
    exactPayloadAvailable: false,
    exactStatusAvailable: false,
    notes: [
      'Exact request latency is unavailable in passive mode.',
      'Exact token counts are unavailable in passive mode.',
      'Exact request and response payloads are unavailable in passive mode.',
      'Exact HTTP status results are unavailable in passive mode.',
    ],
  },
};
