/**
 * TokenRateBadgeProps defines the read-only boundary for the TokenRateBadge atom.
 */
export interface TokenRateBadgeProps {
  /**
   * Tokens per second derived from the terminal done:true NDJSON line.
   * Null means the metric is genuinely unavailable — NOT zero.
   */
  readonly perSec: number | null;
}
