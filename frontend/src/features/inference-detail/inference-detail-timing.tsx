import { PHASE_IN_PROGRESS } from '../../shared/contracts/dashboard-snapshot.types';
import { deriveDisplayTiming } from '../../shared/helpers/live-timing.helpers';
import { useElapsedNow } from '../../shared/hooks/use-elapsed-now';
import { TimingBar } from '../../shared/ui/atoms/timing-bar';
import { formatTimingMilliseconds } from './inference-detail-timing.helpers';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailTiming renders the request timing waterfall and per-phase
 * durations (the Timing tab). Once completed, durations come from the measured
 * counters. While IN PROGRESS the total ticks up with the real elapsed
 * wall-clock (an honest live measurement); the load/eval phases stay "—" because
 * the split is unknown until completion — never fabricated.
 */
export function InferenceDetailTiming({ event }: Readonly<InferenceDetailBodyTabProps>) {
  const live = event.tokens === null && event.status === PHASE_IN_PROGRESS;
  const nowMS = useElapsedNow(live);
  const { loadMS, evalMS, totalMS } = deriveDisplayTiming(event, nowMS);

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
