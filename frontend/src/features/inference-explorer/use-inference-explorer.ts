import { useEffect, useMemo } from 'react';

import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { connectInferenceStore, useInferenceStore } from '../../shared/store/inference-store';
import {
  computeAggregates,
  selectCaptureUnavailable,
  selectFilteredEvents,
} from '../../shared/store/inference-store.helpers';
import { DEFAULT_CAPTURE_NOTE } from './inference-explorer.constants';
import type { UseInferenceExplorerResult } from './inference-explorer.types';

/**
 * useInferenceExplorer wires the Zustand inference store into the explorer view.
 * It establishes the single source->store bridge, then exposes filtered rows,
 * selection, filter state, derived aggregates, and the mutating handlers.
 */
export function useInferenceExplorer(source?: DashboardSnapshotSource): UseInferenceExplorerResult {
  const events = useInferenceStore((state) => state.events);
  const selectedId = useInferenceStore((state) => state.selectedId);
  const query = useInferenceStore((state) => state.query);
  const statusFilter = useInferenceStore((state) => state.statusFilter);
  const onQueryChange = useInferenceStore((state) => state.setQuery);
  const onStatusFilterChange = useInferenceStore((state) => state.setStatusFilter);
  const onSelect = useInferenceStore((state) => state.select);
  const captureMode = useInferenceStore((state) => state.captureMode);
  const passiveNotes = useInferenceStore((state) => state.passiveNotes);

  const rows = useMemo(
    () => selectFilteredEvents(events, query, statusFilter),
    [events, query, statusFilter],
  );
  const aggregates = useMemo(() => computeAggregates(rows), [rows]);
  const captureUnavailable = selectCaptureUnavailable(events, captureMode);
  const captureNote = passiveNotes[passiveNotes.length - 1] ?? DEFAULT_CAPTURE_NOTE;

  // Single bridge for the session — intentionally not torn down on unmount so a
  // remount (or a second consumer) never disconnects a still-mounted view.
  // Placed after derived state to satisfy the project's hook-anatomy rule.
  useEffect(() => {
    connectInferenceStore(source);
  }, [source]);

  return {
    rows,
    selectedId,
    query,
    statusFilter,
    aggregates,
    captureUnavailable,
    captureNote,
    onQueryChange,
    onStatusFilterChange,
    onSelect,
  };
}
