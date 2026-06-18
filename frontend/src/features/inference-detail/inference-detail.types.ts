import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { HttpHeader, InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';

/**
 * InferenceDetailTabKey enumerates the DevTools-style detail tabs (R3).
 * `generation` is the LLM-aware view of an Ollama generate/chat response body.
 */
export type InferenceDetailTabKey = 'overview' | 'payload' | 'response' | 'generation' | 'headers' | 'timing';

/**
 * GenerationView is the parsed, display-ready facet of an Ollama generate/chat
 * response body. The wire body is plain JSON (NOT encrypted): `response` is a
 * (possibly JSON) string and `context` is an array of tokenizer IDs we
 * summarise rather than dump. Built by buildGenerationView; null when the body
 * is absent or is not a generation payload.
 */
export interface GenerationView {
  /** Model output: the `response` field, re-indented when it is itself JSON. */
  readonly output: string;
  /** The verbatim `response` field, before any pretty-printing (for the Raw view + Copy). */
  readonly outputRaw: string;
  /** True when `output` was valid JSON we pretty-printed; false for plain text. */
  readonly outputIsJson: boolean;
  /** Token count of the `context` array, or null when context is absent. */
  readonly contextTokenCount: number | null;
  /** Short, comma-joined preview of the first context token IDs (ellipsis when long). */
  readonly contextPreview: string;
  /** The `done_reason` field, or null when absent. */
  readonly doneReason: string | null;
}

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
 * InferenceDetailCodeBlockProps is the boundary for the shared code viewer used
 * by Payload, Response and the Generation output. Renders `raw` verbatim, offers
 * a Pretty/Raw toggle only when `pretty` is provided, and always exposes a Copy
 * button that copies the verbatim `raw` text.
 */
export interface InferenceDetailCodeBlockProps {
  /** Verbatim body text. Always copyable; shown when Raw is selected. */
  readonly raw: string;
  /** Pretty-printed form. When present, enables the Pretty/Raw toggle (Pretty default). */
  readonly pretty?: string | null;
  /** Whether the underlying body was truncated at the capture byte cap. */
  readonly truncated?: boolean;
}

/**
 * InferenceDetailContextToggleProps is the boundary for the context disclosure
 * control (count label + rotating chevron). Presentational: the parent owns the
 * open state and the toggle handler.
 */
export interface InferenceDetailContextToggleProps {
  /** Text shown beside the chevron (e.g. "521 tokens"). */
  readonly label: string;
  /** Whether the disclosure is expanded (rotates the chevron). */
  readonly open: boolean;
  /** Invoked when the control is activated. */
  readonly onToggle: () => void;
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
