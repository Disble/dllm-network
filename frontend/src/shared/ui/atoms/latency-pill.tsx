import type { LatencyPillProps } from './latency-pill.types';

/**
 * LatencyPill renders end-to-end request latency in milliseconds.
 * Renders an honest em-dash when data is unavailable (null) — never shows fabricated zero.
 */
export function LatencyPill({ latencyMS }: Readonly<LatencyPillProps>) {
  if (latencyMS === null) {
    return (
      <span className="latency-pill latency-pill--unavailable" aria-label="Latency unavailable">
        {'—'}
      </span>
    );
  }

  const formatted = `${Math.round(latencyMS)}ms`;

  return (
    <span className="latency-pill" aria-label={`${Math.round(latencyMS)}ms latency`}>
      {formatted}
    </span>
  );
}
