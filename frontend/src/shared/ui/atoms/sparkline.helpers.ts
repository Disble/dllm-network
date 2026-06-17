import { SPARKLINE_PADDING } from './sparkline.constants';

/**
 * buildPolylinePoints converts a numeric series into an SVG polyline points attribute string.
 * Returns null when there are fewer than 2 values (no line possible).
 */
export const buildPolylinePoints = (values: readonly number[], width: number, height: number): string | null => {
  if (values.length < 2) {
    return null;
  }

  const minVal = Math.min(...values);
  const maxVal = Math.max(...values);
  const range = maxVal - minVal;

  const innerWidth = width - SPARKLINE_PADDING * 2;
  const innerHeight = height - SPARKLINE_PADDING * 2;

  const coords = values.map((value, index) => {
    const x = SPARKLINE_PADDING + (index / (values.length - 1)) * innerWidth;
    // When all values are equal (range === 0), place the line in the middle.
    const normalised = range === 0 ? 0.5 : (value - minVal) / range;
    // SVG y-axis is flipped: high values near top (low y).
    const y = SPARKLINE_PADDING + (1 - normalised) * innerHeight;

    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });

  return coords.join(' ');
};
