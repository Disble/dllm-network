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

  const tokenRateLabel = event.tokens === null
    ? UNAVAILABLE_LABEL
    : `${event.tokens.perSec.toFixed(1)} tok/s`;

  const latencyLabel = event.tokens === null
    ? UNAVAILABLE_LABEL
    : `${Math.round(event.tokens.latencyMS)}ms`;

  const promptEvalCountLabel = event.tokens === null
    ? UNAVAILABLE_LABEL
    : String(event.tokens.promptEvalCount);

  const evalCountLabel = event.tokens === null
    ? UNAVAILABLE_LABEL
    : String(event.tokens.evalCount);

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
