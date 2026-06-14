import { createDashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import { createDashboardViewModel } from './dashboard-view-model.helpers';
import { useDashboardSnapshot } from './use-dashboard-snapshot';
import type { DashboardViewModel } from './dashboard-view-model.types';

/**
 * useDashboardScreen builds the passive dashboard view model from the runtime snapshot source.
 */
export function useDashboardScreen(): DashboardViewModel {
  const snapshot = useDashboardSnapshot({
    source: createDashboardSnapshotSource(),
    initialSnapshot: EMPTY_DASHBOARD_SNAPSHOT,
  });

  return createDashboardViewModel(snapshot, new Date());
}
