import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import { formatBytes, formatTimestamp } from '../../shared/helpers/formatters.helpers';
import { INFERENCE_DETAIL_STATUS_LABELS, UNAVAILABLE_LABEL } from './inference-detail.constants';
import type { InferenceDetailViewModel } from './inference-detail.types';

/**
 * buildInferenceDetailViewModel maps a raw InferenceEvent into a fully display-ready view model.
 * Does NOT fabricate metrics when tokens is null or phase is in-progress.
 */
export function buildInferenceDetailViewModel(event: InferenceEvent): InferenceDetailViewModel {
  const statusLabel = INFERENCE_DETAIL_STATUS_LABELS[event.status] ?? 'unknown';
  const promptSizeLabel = formatBytes(event.promptSize);
  const timestampLabel = formatTimestamp(event.at);

  const tokenRateLabel = event.tokens != null
    ? `${event.tokens.perSec.toFixed(1)} tok/s`
    : UNAVAILABLE_LABEL;

  const latencyLabel = event.tokens != null
    ? `${Math.round(event.tokens.latencyMS)}ms`
    : UNAVAILABLE_LABEL;

  const promptEvalCountLabel = event.tokens != null
    ? String(event.tokens.promptEvalCount)
    : UNAVAILABLE_LABEL;

  const evalCountLabel = event.tokens != null
    ? String(event.tokens.evalCount)
    : UNAVAILABLE_LABEL;

  return {
    model: event.model,
    endpoint: event.endpoint,
    method: event.method,
    statusLabel,
    promptSizeLabel,
    tokenRateLabel,
    latencyLabel,
    promptEvalCountLabel,
    evalCountLabel,
    timestampLabel,
  };
}
