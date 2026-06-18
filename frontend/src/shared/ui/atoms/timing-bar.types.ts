/**
 * TimingBarProps is the read-only boundary for the request-timing atom. All
 * durations are in milliseconds. null means unavailable (null != zero).
 */
export interface TimingBarProps {
  /** Model load phase duration (ms), or null when unavailable. */
  readonly loadMS: number | null;
  /** Token evaluation phase duration (ms), or null when unavailable. */
  readonly evalMS: number | null;
  /** End-to-end total duration (ms), or null when unavailable. */
  readonly totalMS: number | null;
}
