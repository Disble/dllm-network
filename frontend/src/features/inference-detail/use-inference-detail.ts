import { useEffect, useState } from 'react';

import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { InferenceDetailSource } from '../../infrastructure/inference-detail-source';
import { inferenceDetailSource } from '../../infrastructure/inference-detail-source';
import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import { connectInferenceStore, useInferenceStore } from '../../shared/store/inference-store';
import { selectEventById } from '../../shared/store/inference-store.helpers';
import { buildInferenceDetailViewModel } from './inference-detail-view-model.helpers';
import type { UseInferenceDetailResult } from './inference-detail.types';

/**
 * useInferenceDetail returns the SELECTED inference event (master-detail, R2)
 * and its precomputed Overview view model. Selection is read from the shared
 * Zustand store.
 *
 * The recent list in each snapshot carries metadata only; the full record
 * (request/response bodies + headers) is fetched on demand from the durable
 * store when a row is selected — like DevTools loading a body lazily. While the
 * fetch is in flight, or for in-progress rows that are not yet persisted, the
 * hook falls back to the live store event (which still carries the in-progress
 * body), so the panel always has something honest to show.
 */
export function useInferenceDetail(
  source?: DashboardSnapshotSource,
  detailSource: InferenceDetailSource = inferenceDetailSource,
): UseInferenceDetailResult {
  useEffect(() => {
    connectInferenceStore(source);
  }, [source]);

  const events = useInferenceStore((state) => state.events);
  const selectedId = useInferenceStore((state) => state.selectedId);
  const storeEvent = selectEventById(events, selectedId);

  const [detail, setDetail] = useState<InferenceEvent | null>(null);
  useEffect(() => {
    // The detail is fetched asynchronously from the durable store. We
    // intentionally drop the previous row's detail immediately when the
    // selection changes so the panel never renders one row's metadata with
    // another row's body while the new fetch is in flight. This is a classic
    // async-load pattern; deriving state would require blocking the UI.
    // eslint-disable-next-line react-doctor/no-adjust-state-on-prop-change
    setDetail(null);
    if (selectedId === null) {
      return;
    }

    let active = true;
    void detailSource.fetchDetail(selectedId).then((record) => {
      if (active) {
        setDetail(record);
      }
    });

    return () => {
      active = false;
    };
  }, [selectedId, detailSource]);

  const event = detail ?? storeEvent;
  const overview = event === null ? null : buildInferenceDetailViewModel(event);

  return { event, overview };
}
