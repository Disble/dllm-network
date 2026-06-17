import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';
import { formatBytes, formatTimestamp } from '../../shared/helpers/formatters.helpers';
import type { DashboardViewModel } from './dashboard-view-model.types';

const STALE_AFTER_MS = 3 * 60 * 1000;

// ---------------------------------------------------------------------------
// Per-section builder helpers (REFACTOR: split from createDashboardViewModel)
// ---------------------------------------------------------------------------

/**
 * buildConfirmedViewModel extracts the confirmed-state display values from a snapshot.
 */
export const buildConfirmedViewModel = (snapshot: DashboardSnapshot) => ({
  primaryModelValue: snapshot.confirmed.ollama.primaryModel === '' ? 'No confirmed running model' : snapshot.confirmed.ollama.primaryModel,
  ollamaVersionValue: snapshot.confirmed.ollama.version === '' ? 'Unavailable' : snapshot.confirmed.ollama.version,
  processValue: snapshot.confirmed.system.process.found
    ? `PID ${snapshot.confirmed.system.process.pid} • ${snapshot.confirmed.system.process.cpuPercent.toFixed(1)}% CPU • ${formatBytes(snapshot.confirmed.system.process.rssBytes)} RSS`
    : 'Process unavailable',
  connectionsValue: `${snapshot.confirmed.system.connections.count} confirmed loopback connection${snapshot.confirmed.system.connections.count === 1 ? '' : 's'}`,
  hostValue: `${snapshot.confirmed.system.host.cpuPercent.toFixed(1)}% CPU • ${formatBytes(snapshot.confirmed.system.host.memoryUsedBytes)} / ${formatBytes(snapshot.confirmed.system.host.memoryTotalBytes)} memory`,
  observedAtLabel: formatTimestamp(snapshot.confirmed.system.observedAt),
  recentModels: snapshot.recent.confirmedModels.map((entry) => `${entry.model || 'Unknown model'} • ${formatTimestamp(entry.observedAt)}`),
});

/**
 * buildInferredViewModel extracts the inferred-activity display values from a snapshot.
 */
export const buildInferredViewModel = (snapshot: DashboardSnapshot) => {
  const currentInference = snapshot.inferred.current;
  const evidence = currentInference.evidence.map((item) => item.detail);

  return {
    inferredSummary: currentInference.model === '' ? currentInference.kind : `${currentInference.kind} • ${currentInference.model}`,
    confidenceLabel: `${currentInference.confidence} confidence`,
    evidence,
  };
};

/**
 * buildPassiveViewModel extracts the passive-limit display values from a snapshot.
 */
export const buildPassiveViewModel = (snapshot: DashboardSnapshot) => ({
  passiveLimitations: [...snapshot.passive.notes],
});

// ---------------------------------------------------------------------------
// Composite builder (delegates to per-section helpers)
// ---------------------------------------------------------------------------

/**
 * createDashboardViewModel maps a backend dashboard snapshot into passive-only UI content.
 * Delegates to per-section builders (buildConfirmedViewModel, buildInferredViewModel, buildPassiveViewModel).
 */
export function createDashboardViewModel(snapshot: DashboardSnapshot, now: Date): DashboardViewModel {
  const snapshotAgeMs = snapshot.publishedAt === '' ? Number.POSITIVE_INFINITY : now.getTime() - new Date(snapshot.publishedAt).getTime();

  const confirmed = buildConfirmedViewModel(snapshot);
  const inferred = buildInferredViewModel(snapshot);
  const passive = buildPassiveViewModel(snapshot);

  return {
    confirmedBadgeLabel: 'Confirmed telemetry',
    inferredBadgeLabel: 'Inferred activity',
    ...confirmed,
    ...inferred,
    ...passive,
    stalenessLabel: snapshotAgeMs > STALE_AFTER_MS ? 'Stale passive snapshot' : 'Fresh passive snapshot',
    publishedAtLabel: formatTimestamp(snapshot.publishedAt),
  };
}
