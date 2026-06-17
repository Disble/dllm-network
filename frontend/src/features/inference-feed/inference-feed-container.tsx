import { InferenceRow } from './inference-row';
import { useInferenceScreen } from './use-inference-screen';
import type { InferenceFeedContainerProps } from './inference-feed.types';

/**
 * InferenceFeedContainer is a container component that subscribes to the dashboard snapshot source
 * and presents the live inference feed using InferenceRow presentational components.
 * New entries are appended as snapshots arrive; existing rows are never removed.
 * Shows a capture-unavailable banner when no inference events have been observed.
 */
export function InferenceFeedContainer({ source }: Readonly<InferenceFeedContainerProps>) {
  const { events, captureUnavailable, passiveNotes } = useInferenceScreen(source);

  const lastNote = passiveNotes[passiveNotes.length - 1] ?? 'Run as administrator to enable capture.';

  return (
    <section className="inference-feed" aria-label="Live inference feed">
      {captureUnavailable ? (
        <output className="inference-feed__banner">
          <p>
            Live inference capture is unavailable.{' '}
            {lastNote}
          </p>
        </output>
      ) : null}
      <menu className="inference-feed__rows">
        {events.map((event) => (
          <InferenceRow key={`${event.at}::${event.endpoint}::${event.model}`} event={event} />
        ))}
      </menu>
    </section>
  );
}
