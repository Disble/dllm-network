import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import { useDashboardSnapshot } from '../dashboard/use-dashboard-snapshot';
import { useInferenceFeed } from './use-inference-feed';
import type { InferenceScreenState } from './inference-feed.types';

/**
 * useInferenceScreen composes snapshot subscription and feed accumulation into display state.
 * Provides the infrastructure default (dashboardSnapshotSource) while accepting a test seam.
 */
export function useInferenceScreen(source?: DashboardSnapshotSource): InferenceScreenState {
  const resolvedSource = source ?? dashboardSnapshotSource;

  const snapshot = useDashboardSnapshot({
    source: resolvedSource,
    initialSnapshot: EMPTY_DASHBOARD_SNAPSHOT,
  });

  const events = useInferenceFeed(resolvedSource);

  const captureUnavailable = events.length === 0 && snapshot.passive.mode === 'passive-only';

  return {
    events,
    captureUnavailable,
    passiveNotes: snapshot.passive.notes,
  };
}
