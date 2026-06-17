import { LatencyPill } from '../../shared/ui/atoms/latency-pill';
import { TokenRateBadge } from '../../shared/ui/atoms/token-rate-badge';
import { INFERENCE_STATUS_LABELS } from './inference-feed.constants';
import type { InferenceRowProps } from './inference-feed.types';

/**
 * InferenceRow renders a single inference event as a presentational row.
 * Receives a raw InferenceEvent and derives display values; never imports infrastructure.
 */
export function InferenceRow({ event }: Readonly<InferenceRowProps>) {
  const statusLabel = INFERENCE_STATUS_LABELS[event.status] ?? 'unknown';
  const perSec = event.tokens?.perSec ?? null;
  const latencyMS = event.tokens?.latencyMS ?? null;

  return (
    <article className="inference-row" role="listitem">
      <div className="inference-row__meta">
        <span className="inference-row__model">{event.model}</span>
        <span className="inference-row__endpoint">{event.endpoint}</span>
        <span className="inference-row__status">{statusLabel}</span>
      </div>
      <div className="inference-row__metrics">
        <TokenRateBadge perSec={perSec} />
        <LatencyPill latencyMS={latencyMS} />
      </div>
    </article>
  );
}
