import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';
import { formatBytes, formatTimestamp } from '../../shared/helpers/formatters.helpers';
import type { DashboardViewModel } from './dashboard-view-model.types';

const STALE_AFTER_MS = 3 * 60 * 1000;

/**
 * createDashboardViewModel maps a backend dashboard snapshot into passive-only UI content.
 */
export function createDashboardViewModel(snapshot: DashboardSnapshot, now: Date): DashboardViewModel {
  const snapshotAgeMs = snapshot.publishedAt === '' ? Number.POSITIVE_INFINITY : now.getTime() - new Date(snapshot.publishedAt).getTime();
  const currentInference = snapshot.inferred.current;
  const evidence = currentInference.evidence.map((item) => item.detail);

  return {
    confirmedBadgeLabel: 'Confirmed telemetry',
    inferredBadgeLabel: 'Inferred activity',
    primaryModelValue: snapshot.confirmed.ollama.primaryModel === '' ? 'No confirmed running model' : snapshot.confirmed.ollama.primaryModel,
    ollamaVersionValue: snapshot.confirmed.ollama.version === '' ? 'Unavailable' : snapshot.confirmed.ollama.version,
    processValue: snapshot.confirmed.system.process.found
      ? `PID ${snapshot.confirmed.system.process.pid} • ${snapshot.confirmed.system.process.cpuPercent.toFixed(1)}% CPU • ${formatBytes(snapshot.confirmed.system.process.rssBytes)} RSS`
      : 'Process unavailable',
    connectionsValue: `${snapshot.confirmed.system.connections.count} confirmed loopback connection${snapshot.confirmed.system.connections.count === 1 ? '' : 's'}`,
    hostValue: `${snapshot.confirmed.system.host.cpuPercent.toFixed(1)}% CPU • ${formatBytes(snapshot.confirmed.system.host.memoryUsedBytes)} / ${formatBytes(snapshot.confirmed.system.host.memoryTotalBytes)} memory`,
    inferredSummary: currentInference.model === '' ? currentInference.kind : `${currentInference.kind} • ${currentInference.model}`,
    confidenceLabel: `${currentInference.confidence} confidence`,
    evidence,
    passiveLimitations: [...snapshot.passive.notes],
    stalenessLabel: snapshotAgeMs > STALE_AFTER_MS ? 'Stale passive snapshot' : 'Fresh passive snapshot',
    publishedAtLabel: formatTimestamp(snapshot.publishedAt),
    observedAtLabel: formatTimestamp(snapshot.confirmed.system.observedAt),
    recentModels: snapshot.recent.confirmedModels.map((entry) => `${entry.model || 'Unknown model'} • ${formatTimestamp(entry.observedAt)}`),
  };
}
