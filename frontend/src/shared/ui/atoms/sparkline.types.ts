/**
 * SparklineProps defines the read-only boundary for the Sparkline atom.
 */
export interface SparklineProps {
  /** Numeric series to plot. Must have >= 2 entries for a visible line. */
  readonly values: readonly number[];
  /** SVG width in pixels. Defaults to 80. */
  readonly width?: number;
  /** SVG height in pixels. Defaults to 24. */
  readonly height?: number;
  /** Optional aria-label for accessibility. */
  readonly ariaLabel?: string;
}
