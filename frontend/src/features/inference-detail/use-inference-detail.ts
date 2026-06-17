import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import { useDashboardSnapshot } from '../dashboard/use-dashboard-snapshot';
import { buildInferenceDetailViewModel } from './inference-detail-view-model.helpers';
import type { InferenceDetailViewModel } from './inference-detail.types';

/**
 * useInferenceDetail subscribes to the snapshot source and returns the precomputed
 * view model for the most recent (current) inference event.
 */
export function useInferenceDetail(source?: DashboardSnapshotSource): InferenceDetailViewModel {
  const resolvedSource = source ?? dashboardSnapshotSource;

  const snapshot = useDashboardSnapshot({
    source: resolvedSource,
    initialSnapshot: EMPTY_DASHBOARD_SNAPSHOT,
  });

  return buildInferenceDetailViewModel(snapshot.inference.current);
}
