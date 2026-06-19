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
  /** Largest latency (ms) across the visible rows — the waterfall scale reference. */
  readonly maxLatencyMS: number;
  /** Current epoch-ms clock, so in-progress rows render live elapsed timing. */
  readonly nowMS: number;
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

/**
 * InferenceKpiItem is one display-ready cell of the top KPI strip.
 */
export interface InferenceKpiItem {
  /** Uppercase metric label (e.g. "AVG TOK/S"). */
  readonly label: string;
  /** Formatted metric value (e.g. "45.0 tok/s" or "—"). */
  readonly value: string;
  /** Sub-caption beneath the value (e.g. "Average"). */
  readonly caption: string;
}

/**
 * UseInferenceMetricsResult is the view model for the KPI strip hook.
 */
export interface UseInferenceMetricsResult {
  /** Display-ready KPI cells in display order. */
  readonly items: readonly InferenceKpiItem[];
}

/**
 * InferenceKpiStripProps is the boundary for the top KPI strip presentational.
 */
export interface InferenceKpiStripProps {
  /** Display-ready KPI cells. */
  readonly items: readonly InferenceKpiItem[];
}
