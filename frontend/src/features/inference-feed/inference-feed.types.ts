import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';

/**
 * InferenceFeedContainerProps defines the injectable source seam for the inference feed container.
 */
export interface InferenceFeedContainerProps {
  /** Runtime snapshot source. Defaults to the shared infrastructure source in production. */
  readonly source?: DashboardSnapshotSource;
}

/**
 * InferenceRowProps defines the read-only boundary for the inference row presentational component.
 * The row receives a raw InferenceEvent and derives its display values internally.
 */
export interface InferenceRowProps {
  readonly event: InferenceEvent;
}

/**
 * InferenceScreenState carries the display state for the inference feed screen.
 */
export interface InferenceScreenState {
  readonly events: readonly InferenceEvent[];
  readonly captureUnavailable: boolean;
  readonly passiveNotes: readonly string[];
}
