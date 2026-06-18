/**
 * Tests for the WaterfallBar atom and its pure geometry helper. The bar's total
 * WIDTH is scaled against the slowest request in the visible set (so rows are
 * visually comparable), while the internal load/eval/other SEGMENTS are
 * proportions of that row's own total. Honest null when timing is unavailable —
 * we never fabricate phases we did not measure.
 */
import { createElement } from 'react';
import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { WaterfallBar } from '../waterfall-bar';
import { buildWaterfallGeometry } from '../waterfall-bar.helpers';

afterEach(() => {
  cleanup();
});

describe('buildWaterfallGeometry', () => {
  it('returns null when the total is unavailable', () => {
    expect(buildWaterfallGeometry({ loadMS: 10, evalMS: 20, totalMS: null, maxMS: 100 })).toBeNull();
  });

  it('returns null when the scale reference is non-positive', () => {
    expect(buildWaterfallGeometry({ loadMS: 10, evalMS: 20, totalMS: 50, maxMS: 0 })).toBeNull();
  });

  it('scales bar width against the slowest request', () => {
    const geometry = buildWaterfallGeometry({ loadMS: 100, evalMS: 4000, totalMS: 4200, maxMS: 8400 });
    expect(geometry?.barWidthPct).toBeCloseTo(50, 1);
  });

  it('splits the bar into load/eval/other proportions of the row total', () => {
    const geometry = buildWaterfallGeometry({ loadMS: 100, evalMS: 4000, totalMS: 4200, maxMS: 4200 });
    expect(geometry?.barWidthPct).toBeCloseTo(100, 1);
    expect(geometry?.loadPct).toBeCloseTo((100 / 4200) * 100, 1);
    expect(geometry?.evalPct).toBeCloseTo((4000 / 4200) * 100, 1);
    expect(geometry?.otherPct).toBeCloseTo((100 / 4200) * 100, 1);
  });

  it('treats missing phase durations as zero (no fabricated segments)', () => {
    const geometry = buildWaterfallGeometry({ loadMS: null, evalMS: null, totalMS: 1000, maxMS: 1000 });
    expect(geometry?.loadPct).toBe(0);
    expect(geometry?.evalPct).toBe(0);
    expect(geometry?.otherPct).toBeCloseTo(100, 1);
  });

  it('never produces negative other when phases exceed total (clamps)', () => {
    const geometry = buildWaterfallGeometry({ loadMS: 800, evalMS: 800, totalMS: 1000, maxMS: 1000 });
    expect(geometry?.otherPct).toBe(0);
  });
});

describe('WaterfallBar', () => {
  it('renders an honest em-dash when timing is unavailable', () => {
    const { container } = render(
      createElement(WaterfallBar, { loadMS: null, evalMS: null, totalMS: null, maxMS: 100 }),
    );
    expect(container.textContent).toContain('—');
    expect(container.querySelector('.waterfall-bar__segment')).toBeNull();
  });

  it('renders the three phase segments when timing is available', () => {
    const { container } = render(
      createElement(WaterfallBar, { loadMS: 100, evalMS: 4000, totalMS: 4200, maxMS: 8400 }),
    );
    expect(container.querySelectorAll('.waterfall-bar__segment')).toHaveLength(3);
  });
});
