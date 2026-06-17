import { PHASE_COMPLETED, PHASE_IN_PROGRESS, PHASE_METADATA_ONLY } from '../../shared/contracts/dashboard-snapshot.types';

/**
 * INFERENCE_STATUS_LABELS maps InferencePhase values to human-readable status strings.
 */
export const INFERENCE_STATUS_LABELS: Readonly<Record<number, string>> = {
  [PHASE_IN_PROGRESS]: 'in progress',
  [PHASE_COMPLETED]: 'completed',
  [PHASE_METADATA_ONLY]: 'metadata only',
} as const;
