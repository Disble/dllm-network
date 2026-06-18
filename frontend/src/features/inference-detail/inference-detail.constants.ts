import { PHASE_COMPLETED, PHASE_IN_PROGRESS, PHASE_METADATA_ONLY } from '../../shared/contracts/dashboard-snapshot.types';
import type { InferenceDetailTabKey } from './inference-detail.types';

/**
 * INFERENCE_DETAIL_STATUS_LABELS maps InferencePhase values to human-readable status strings
 * for the detail view.
 */
export const INFERENCE_DETAIL_STATUS_LABELS: Readonly<Record<number, string>> = {
  [PHASE_IN_PROGRESS]: 'in progress',
  [PHASE_COMPLETED]: 'completed',
  [PHASE_METADATA_ONLY]: 'metadata only',
} as const;

/** Sentinel for unavailable metric fields. */
export const UNAVAILABLE_LABEL = '—';

/** How long the code-block "Copied" confirmation stays visible after a copy (ms). */
export const COPIED_RESET_MS = 1500;

/**
 * NOT_CAPTURED_LABEL is shown in body/headers tabs when passive capture has not
 * surfaced the data. TECH DEBT (Slice A backend): the capture pipeline parses
 * bodies + headers at the wire but the extractor currently discards them, so
 * these tabs stay empty until that plumbing lands. "not captured" != empty.
 */
export const NOT_CAPTURED_LABEL = 'Not captured in passive mode yet.';

/**
 * INFERENCE_DETAIL_TABS defines the DevTools-style detail tabs in display order (R3).
 */
export const INFERENCE_DETAIL_TABS: readonly { readonly key: InferenceDetailTabKey; readonly label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'payload', label: 'Payload' },
  { key: 'response', label: 'Response' },
  { key: 'generation', label: 'Generation' },
  { key: 'headers', label: 'Headers' },
  { key: 'timing', label: 'Timing' },
] as const;
