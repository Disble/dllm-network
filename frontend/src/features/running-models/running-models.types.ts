import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';

/**
 * RunningModelCardViewModel holds precomputed display fields for a single running model card.
 * All numeric fields are pre-formatted as strings; callers never format raw numbers.
 */
export interface RunningModelCardViewModel {
  readonly name: string;
  /** e.g. "8B" or "—" when absent. */
  readonly parameterSize: string;
  /** e.g. "Q4_0" or "—" when absent. */
  readonly quantizationLevel: string;
  /** Human-readable byte size, e.g. "4.2 GB". */
  readonly sizeLabel: string;
  /** Human-readable VRAM footprint, e.g. "3.9 GB". */
  readonly sizeVramLabel: string;
  /** Context window length as a formatted string, e.g. "8192". */
  readonly contextLengthLabel: string;
  /** Relative or absolute expires_at label, e.g. "in 2m 30s" or "—" when absent. */
  readonly expiresAtLabel: string;
}

/**
 * RunningModelCardProps defines the read-only boundary for the card presentational component.
 */
export interface RunningModelCardProps {
  readonly viewModel: RunningModelCardViewModel;
}

/**
 * RunningModelsContainerProps defines the injectable source seam for the running-models container.
 */
export interface RunningModelsContainerProps {
  /** Runtime snapshot source. Defaults to the shared infrastructure source in production. */
  readonly source?: DashboardSnapshotSource;
}
