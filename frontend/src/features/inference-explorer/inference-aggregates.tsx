import type { InferenceAggregatesProps } from './inference-explorer.types';

/**
 * InferenceAggregates renders the summary header: request count, average tok/s,
 * and p50/p95 latency over the filtered set (R5). Unavailable metrics render an
 * honest em-dash rather than a fabricated zero.
 */
export function InferenceAggregates({ aggregates }: Readonly<InferenceAggregatesProps>) {
  const avg = aggregates.avgPerSec === null ? '—' : `${aggregates.avgPerSec.toFixed(1)} tok/s`;
  const p50 = aggregates.p50LatencyMS === null ? '—' : `${Math.round(aggregates.p50LatencyMS)}ms`;
  const p95 = aggregates.p95LatencyMS === null ? '—' : `${Math.round(aggregates.p95LatencyMS)}ms`;

  return (
    <dl className="inference-aggregates">
      <div className="inference-aggregates__item">
        <dt>Requests</dt>
        <dd>{aggregates.count}</dd>
      </div>
      <div className="inference-aggregates__item">
        <dt>Avg tok/s</dt>
        <dd>{avg}</dd>
      </div>
      <div className="inference-aggregates__item">
        <dt>p50 latency</dt>
        <dd>{p50}</dd>
      </div>
      <div className="inference-aggregates__item">
        <dt>p95 latency</dt>
        <dd>{p95}</dd>
      </div>
    </dl>
  );
}
