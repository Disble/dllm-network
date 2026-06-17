/**
 * LatencyPillProps defines the read-only boundary for the LatencyPill atom.
 */
export interface LatencyPillProps {
  /**
   * End-to-end latency in milliseconds (total_duration converted from nanoseconds).
   * Null means the metric is genuinely unavailable — NOT zero.
   */
  readonly latencyMS: number | null;
}
