import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';

/**
 * InferenceDetailViewModel holds the precomputed display fields for the detail panel.
 * All fields are plain strings; callers never format raw numbers directly.
 */
export interface InferenceDetailViewModel {
  readonly model: string;
  readonly endpoint: string;
  readonly method: string;
  readonly statusLabel: string;
  readonly promptSizeLabel: string;
  readonly tokenRateLabel: string;
  readonly latencyLabel: string;
  readonly promptEvalCountLabel: string;
  readonly evalCountLabel: string;
  readonly timestampLabel: string;
}

/**
 * InferenceDetailPanelProps defines the read-only boundary for the detail panel presentational component.
 */
export interface InferenceDetailPanelProps {
  readonly viewModel: InferenceDetailViewModel;
}

/**
 * InferenceDetailContainerProps defines the injectable source seam for the detail container.
 */
export interface InferenceDetailContainerProps {
  /** Runtime snapshot source. Defaults to the shared infrastructure source in production. */
  readonly source?: DashboardSnapshotSource;
}
