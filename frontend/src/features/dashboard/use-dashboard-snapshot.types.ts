import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';

/**
 * UseDashboardSnapshotOptions defines the source boundary for passive dashboard event consumption.
 */
export interface UseDashboardSnapshotOptions {
  readonly source: DashboardSnapshotSource;
  readonly initialSnapshot: DashboardSnapshot;
}
