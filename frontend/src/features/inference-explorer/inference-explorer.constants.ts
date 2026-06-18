import { PHASE_COMPLETED, PHASE_IN_PROGRESS, PHASE_METADATA_ONLY } from '../../shared/contracts/dashboard-snapshot.types';
import type { InferenceStatusFilter } from '../../shared/store/inference-store.types';

/**
 * INFERENCE_ROW_HEIGHT is the fixed virtualized row height in pixels. Fixed
 * height keeps the @tanstack/react-virtual estimate exact and scroll smooth.
 */
export const INFERENCE_ROW_HEIGHT = 56;

/**
 * INFERENCE_TABLE_OVERSCAN is how many off-screen rows the virtualizer keeps
 * mounted above/below the viewport to avoid blank flashes during fast scroll.
 */
export const INFERENCE_TABLE_OVERSCAN = 8;

/**
 * INFERENCE_STATUS_LABELS maps a lifecycle phase to a short human label.
 */
export const INFERENCE_STATUS_LABELS: Readonly<Record<number, string>> = {
  [PHASE_IN_PROGRESS]: 'in progress',
  [PHASE_COMPLETED]: 'completed',
  [PHASE_METADATA_ONLY]: 'metadata',
} as const;

/**
 * INFERENCE_STATUS_FILTER_OPTIONS drives the status filter control, in display order.
 */
export const INFERENCE_STATUS_FILTER_OPTIONS: readonly { readonly value: InferenceStatusFilter; readonly label: string }[] = [
  { value: 'all', label: 'All' },
  { value: PHASE_COMPLETED, label: 'Completed' },
  { value: PHASE_IN_PROGRESS, label: 'In progress' },
  { value: PHASE_METADATA_ONLY, label: 'Metadata' },
] as const;
