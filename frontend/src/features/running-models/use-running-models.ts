import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import { useDashboardSnapshot } from '../dashboard/use-dashboard-snapshot';
import { buildRunningModelCardViewModel } from './running-model-card-view-model.helpers';
import type { RunningModelCardViewModel } from './running-models.types';

/**
 * useRunningModels subscribes to the snapshot source and returns view models
 * for all currently running models from the confirmed Ollama state.
 */
export function useRunningModels(source?: DashboardSnapshotSource): readonly RunningModelCardViewModel[] {
  const resolvedSource = source ?? dashboardSnapshotSource;

  const snapshot = useDashboardSnapshot({
    source: resolvedSource,
    initialSnapshot: EMPTY_DASHBOARD_SNAPSHOT,
  });

  const details = snapshot.confirmed.ollama.runningModelDetails ?? [];

  return details.map(buildRunningModelCardViewModel);
}
