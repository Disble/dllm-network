import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { HttpHeader, InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';

/**
 * InferenceDetailTabKey enumerates the DevTools-style detail tabs (R3).
 */
export type InferenceDetailTabKey = 'overview' | 'payload' | 'response' | 'headers' | 'timing';

/**
 * InferenceDetailViewModel holds the precomputed Overview-tab fields.
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
 * UseInferenceDetailResult is the detail hook output: the selected event (or
 * null when nothing is selected) and its precomputed Overview view model.
 */
export interface UseInferenceDetailResult {
  /** The currently selected event, or null when no row is selected. */
  readonly event: InferenceEvent | null;
  /** Overview-tab view model, or null when no event is selected. */
  readonly overview: InferenceDetailViewModel | null;
}

/**
 * InferenceDetailContainerProps defines the injectable source seam for the detail container.
 */
export interface InferenceDetailContainerProps {
  /** Runtime snapshot source. Defaults to the shared infrastructure source in production. */
  readonly source?: DashboardSnapshotSource;
}

/**
 * InferenceDetailPanelProps is the boundary for the tabbed detail panel.
 */
export interface InferenceDetailPanelProps {
  /** Selected event, or null when none is selected. */
  readonly event: InferenceEvent | null;
  /** Overview view model, or null when no event is selected. */
  readonly overview: InferenceDetailViewModel | null;
}

/**
 * InferenceDetailOverviewProps is the boundary for the Overview tab.
 */
export interface InferenceDetailOverviewProps {
  /** Precomputed overview fields. */
  readonly viewModel: InferenceDetailViewModel;
}

/**
 * InferenceDetailBodyTabProps is the boundary for the Payload, Response, Headers
 * and Timing tabs — each renders some facet of the selected event.
 */
export interface InferenceDetailBodyTabProps {
  /** The selected event whose facet this tab renders. */
  readonly event: InferenceEvent;
}

/**
 * InferenceDetailBodyProps is the boundary for the shared raw-body renderer used
 * by the Payload and Response tabs.
 */
export interface InferenceDetailBodyProps {
  /** Raw body text, or undefined when not captured. */
  readonly body?: string;
  /** Whether the body was truncated at the capture byte cap. */
  readonly truncated?: boolean;
}

/**
 * InferenceDetailHeaderGroupProps is the boundary for one labelled, ordered
 * header table inside the Headers tab.
 */
export interface InferenceDetailHeaderGroupProps {
  /** Group heading (e.g. "Request headers"). */
  readonly title: string;
  /** Ordered headers; the group renders nothing when empty. */
  readonly headers: readonly HttpHeader[];
}
