import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import { createDashboardViewModel } from './dashboard-view-model.helpers';
import { useDashboardSnapshot } from './use-dashboard-snapshot';
import type { UseDashboardScreenOptions } from './use-dashboard-screen.types';
import type { DashboardViewModel } from './dashboard-view-model.types';

/**
 * useDashboardScreen builds the passive dashboard view model from the runtime snapshot source.
 */
export function useDashboardScreen(options: Readonly<UseDashboardScreenOptions> = {}): DashboardViewModel {
  const snapshot = useDashboardSnapshot({
    source: options.source ?? dashboardSnapshotSource,
    initialSnapshot: EMPTY_DASHBOARD_SNAPSHOT,
  });

  return createDashboardViewModel(snapshot, options.now ?? new Date());
}
