/**
 * WaterfallBarProps is the read-only boundary for the table waterfall atom. All
 * durations are in milliseconds; null means unavailable (null != zero). The bar
 * width is scaled against `maxMS` (the slowest request in the visible set).
 */
export interface WaterfallBarProps {
  /** Model load phase duration (ms), or null when unavailable. */
  readonly loadMS: number | null;
  /** Token evaluation phase duration (ms), or null when unavailable. */
  readonly evalMS: number | null;
  /** This row's end-to-end total duration (ms), or null when unavailable. */
  readonly totalMS: number | null;
  /** Scale reference: the largest total across the visible rows (ms). */
  readonly maxMS: number;
}

/**
 * WaterfallGeometry holds the precomputed bar/segment widths as percentages.
 * `barWidthPct` is the bar's share of the column (row total vs the slowest row);
 * the segment percentages split that bar by phase and always sum to <= 100.
 */
export interface WaterfallGeometry {
  /** Bar width as a percentage of the column (row total / maxMS). */
  readonly barWidthPct: number;
  /** Load phase as a percentage of the row total. */
  readonly loadPct: number;
  /** Eval phase as a percentage of the row total. */
  readonly evalPct: number;
  /** Remaining (unattributed) time as a percentage of the row total. */
  readonly otherPct: number;
}

/**
 * WaterfallGeometryInput is the plain numeric input to buildWaterfallGeometry.
 */
export interface WaterfallGeometryInput {
  /** Load phase duration (ms) or null. */
  readonly loadMS: number | null;
  /** Eval phase duration (ms) or null. */
  readonly evalMS: number | null;
  /** Row total duration (ms) or null. */
  readonly totalMS: number | null;
  /** Scale reference (ms). */
  readonly maxMS: number;
}
