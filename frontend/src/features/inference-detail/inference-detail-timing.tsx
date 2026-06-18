import { TimingBar } from '../../shared/ui/atoms/timing-bar';
import { formatTimingMilliseconds } from './inference-detail-timing.helpers';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailTiming renders the request timing waterfall and per-phase
 * durations (the Timing tab). Durations come from the terminal NDJSON counters
 * (nanoseconds) and render an em-dash when no completed metrics exist.
 */
export function InferenceDetailTiming({ event }: Readonly<InferenceDetailBodyTabProps>) {
  const tokens = event.tokens;
  const loadMS = tokens != null ? tokens.loadDuration / 1e6 : null;
  const evalMS = tokens != null ? tokens.evalDuration / 1e6 : null;
  const totalMS = tokens != null ? tokens.latencyMS : null;

  return (
    <div className="inference-detail__timing">
      <TimingBar loadMS={loadMS} evalMS={evalMS} totalMS={totalMS} />
      <dl className="inference-detail__fields">
        <dt>Load</dt>
        <dd>{formatTimingMilliseconds(loadMS)}</dd>
        <dt>Eval</dt>
        <dd>{formatTimingMilliseconds(evalMS)}</dd>
        <dt>Total</dt>
        <dd>{formatTimingMilliseconds(totalMS)}</dd>
      </dl>
    </div>
  );
}
