import { useEffect } from 'react';

import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { connectInferenceStore, useInferenceStore } from '../../shared/store/inference-store';
import { selectEventById } from '../../shared/store/inference-store.helpers';
import { buildInferenceDetailViewModel } from './inference-detail-view-model.helpers';
import type { UseInferenceDetailResult } from './inference-detail.types';

/**
 * useInferenceDetail returns the SELECTED inference event (master-detail, R2)
 * and its precomputed Overview view model. It reads selection from the shared
 * Zustand store rather than hardwiring the most-recent event.
 */
export function useInferenceDetail(source?: DashboardSnapshotSource): UseInferenceDetailResult {
  useEffect(() => {
    connectInferenceStore(source);
  }, [source]);

  const events = useInferenceStore((state) => state.events);
  const selectedId = useInferenceStore((state) => state.selectedId);

  const event = selectEventById(events, selectedId);
  const overview = event === null ? null : buildInferenceDetailViewModel(event);

  return { event, overview };
}
