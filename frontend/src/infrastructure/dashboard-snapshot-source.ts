import { EMPTY_DASHBOARD_SNAPSHOT } from '../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../shared/contracts/dashboard-snapshot.types';

/**
 * DashboardSnapshotSource subscribes to dashboard snapshot updates from a runtime-specific transport.
 */
export interface DashboardSnapshotSource {
  // eslint-disable-next-line no-unused-vars -- Type-only callback parameter documents the snapshot contract.
  readonly subscribe: (listener: (snapshot: DashboardSnapshot) => void) => () => void;
  readonly getSnapshot: () => DashboardSnapshot;
}

/**
 * createDashboardSnapshotSource creates a no-op source until Wails EventsOn wiring is attached.
 */
export function createDashboardSnapshotSource(): DashboardSnapshotSource {
  return {
    subscribe() {
      return () => undefined;
    },
    getSnapshot() {
      return EMPTY_DASHBOARD_SNAPSHOT;
    },
  };
}
