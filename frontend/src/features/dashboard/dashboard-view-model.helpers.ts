import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';
import { formatClockDateTime, formatTimestamp } from '../../shared/helpers/formatters.helpers';
import type { DashboardViewModel } from './dashboard-view-model.types';

const STALE_AFTER_MS = 3 * 60 * 1000;

/**
 * humaniseCollectionMode turns the raw passive mode flag into a tile label.
 */
export function humaniseCollectionMode(mode: string): string {
  if (mode === 'passive-only') {
    return 'Passive-only';
  }
  if (mode === 'capture-active') {
    return 'Capture active';
  }
  return mode === '' ? 'Unknown' : mode;
}

/**
 * createDashboardViewModel maps a backend dashboard snapshot into the compact
 * passive-telemetry summary (freshness + collection mode + snapshot time +
 * health). It deliberately omits the verbose confirmed/inferred/passive-limit
 * detail that previously cluttered the panel.
 */
export function createDashboardViewModel(snapshot: DashboardSnapshot, now: Date): DashboardViewModel {
  const snapshotAgeMs = snapshot.publishedAt === ''
    ? Number.POSITIVE_INFINITY
    : now.getTime() - new Date(snapshot.publishedAt).getTime();
  const isFresh = snapshotAgeMs <= STALE_AFTER_MS;

  return {
    publishedAtLabel: formatTimestamp(snapshot.publishedAt),
    stalenessLabel: isFresh ? 'Fresh passive snapshot' : 'Stale passive snapshot',
    isFresh,
    collectionModeLabel: humaniseCollectionMode(snapshot.passive.mode),
    snapshotTimeLabel: formatClockDateTime(snapshot.publishedAt),
    healthLabel: snapshot.health.ollama.healthy ? 'Healthy' : 'Unavailable',
  };
}
