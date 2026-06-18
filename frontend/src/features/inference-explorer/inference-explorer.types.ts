import type { CSSProperties } from 'react';

import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import type { InferenceAggregates, InferenceStatusFilter } from '../../shared/store/inference-store.types';

/**
 * InferenceExplorerContainerProps defines the injectable source seam for the explorer.
 */
export interface InferenceExplorerContainerProps {
  /** Runtime snapshot source. Defaults to the shared infrastructure source in production. */
  readonly source?: DashboardSnapshotSource;
}

/**
 * UseInferenceExplorerResult is the view model returned by the explorer hook:
 * filtered rows, selection, filter state, derived aggregates, and handlers.
 */
export interface UseInferenceExplorerResult {
  /** Filtered, accumulated events to render as table rows. */
  readonly rows: readonly InferenceEvent[];
  /** Currently selected event id, or null. */
  readonly selectedId: string | null;
  /** Active free-text query. */
  readonly query: string;
  /** Active lifecycle-phase filter. */
  readonly statusFilter: InferenceStatusFilter;
  /** Summary metrics over the filtered set. */
  readonly aggregates: InferenceAggregates;
  /** True when live capture is unavailable (no events + passive-only mode). */
  readonly captureUnavailable: boolean;
  /** Hint shown alongside the capture-unavailable banner. */
  readonly captureNote: string;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onQueryChange: (query: string) => void;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onStatusFilterChange: (filter: InferenceStatusFilter) => void;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onSelect: (id: string) => void;
}

/**
 * InferenceTableProps is the boundary for the virtualized request table.
 */
export interface InferenceTableProps {
  /** Rows to render (already filtered). */
  readonly rows: readonly InferenceEvent[];
  /** Currently selected row id. */
  readonly selectedId: string | null;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onSelect: (id: string) => void;
}

/**
 * InferenceTableRowProps is the boundary for one presentational table row.
 */
export interface InferenceTableRowProps {
  /** The raw event this row renders. */
  readonly event: InferenceEvent;
  /** Stable id for this row (derived if backend omits it). */
  readonly rowId: string;
  /** Whether this row is the selected one. */
  readonly isSelected: boolean;
  /** Absolute positioning style supplied by the virtualizer. */
  readonly style: CSSProperties;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onSelect: (id: string) => void;
}

/**
 * InferenceFilterBarProps is the boundary for the search + status filter controls.
 */
export interface InferenceFilterBarProps {
  /** Current query text. */
  readonly query: string;
  /** Current status filter. */
  readonly statusFilter: InferenceStatusFilter;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onQueryChange: (query: string) => void;
  // eslint-disable-next-line no-unused-vars -- function-type param documents the handler contract.
  readonly onStatusFilterChange: (filter: InferenceStatusFilter) => void;
}

/**
 * InferenceAggregatesProps is the boundary for the summary metrics header.
 */
export interface InferenceAggregatesProps {
  /** Derived summary metrics. */
  readonly aggregates: InferenceAggregates;
}
