import type { WaterfallGeometry, WaterfallGeometryInput } from './waterfall-bar.types';

/**
 * buildWaterfallGeometry converts a row's phase durations into bar/segment
 * widths. Returns null when the total or the scale reference is unavailable —
 * never fabricating a bar for a request we did not fully measure. Phase segments
 * are proportions of the row total; missing phases count as zero and the
 * remainder ("other") is clamped at zero so segments never exceed the bar.
 */
export const buildWaterfallGeometry = ({ loadMS, evalMS, totalMS, maxMS }: WaterfallGeometryInput): WaterfallGeometry | null => {
  if (totalMS === null || totalMS <= 0 || maxMS <= 0) {
    return null;
  }

  const load = loadMS ?? 0;
  const evaluate = evalMS ?? 0;
  const other = Math.max(totalMS - load - evaluate, 0);

  return {
    barWidthPct: Math.min(totalMS / maxMS, 1) * 100,
    loadPct: (load / totalMS) * 100,
    evalPct: (evaluate / totalMS) * 100,
    otherPct: (other / totalMS) * 100,
  };
};
