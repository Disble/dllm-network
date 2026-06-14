import { useSyncExternalStore } from 'react';

import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';
import type { UseDashboardSnapshotOptions } from './use-dashboard-snapshot.types';

/**
 * useDashboardSnapshot keeps the latest passive snapshot from the configured source.
 */
export function useDashboardSnapshot(options: Readonly<UseDashboardSnapshotOptions>): DashboardSnapshot {
  return useSyncExternalStore(options.source.subscribe, options.source.getSnapshot, () => options.initialSnapshot);
}
