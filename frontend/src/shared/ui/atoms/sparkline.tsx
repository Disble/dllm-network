import { SPARKLINE_DEFAULT_HEIGHT, SPARKLINE_DEFAULT_WIDTH } from './sparkline.constants';
import { buildPolylinePoints } from './sparkline.helpers';
import type { SparklineProps } from './sparkline.types';

/**
 * Sparkline is a pure presentational SVG sparkline atom.
 * Receives a series of numeric values and maps them to a bounded SVG polyline.
 * No chart library dependency — hand-rolled path.
 * Renders no polyline when fewer than 2 values are supplied.
 */
export function Sparkline({ values, width = SPARKLINE_DEFAULT_WIDTH, height = SPARKLINE_DEFAULT_HEIGHT, ariaLabel }: Readonly<SparklineProps>) {
  const points = buildPolylinePoints(values, width, height);

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      aria-label={ariaLabel}
      role={ariaLabel === undefined ? undefined : 'img'}
      aria-hidden={ariaLabel === undefined ? true : undefined}
    >
      {points !== null && (
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      )}
    </svg>
  );
}
