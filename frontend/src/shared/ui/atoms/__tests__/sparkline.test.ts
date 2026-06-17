/**
 * Tests for the Sparkline pure SVG atom.
 * No chart library — hand-rolled polyline path.
 */
import { createElement } from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Sparkline } from '../sparkline';

describe('Sparkline', () => {
  it('renders an SVG element', () => {
    const { container } = render(createElement(Sparkline, { values: [1, 2, 3] }));
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
  });

  it('renders a polyline when values are provided', () => {
    const { container } = render(createElement(Sparkline, { values: [10, 20, 15, 30] }));
    const polyline = container.querySelector('polyline');
    expect(polyline).not.toBeNull();
    expect(polyline?.getAttribute('points')).toBeTruthy();
  });

  it('renders no polyline when values array is empty', () => {
    const { container } = render(createElement(Sparkline, { values: [] }));
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    const polyline = container.querySelector('polyline');
    expect(polyline).toBeNull();
  });

  it('renders no polyline for a single value', () => {
    const { container } = render(createElement(Sparkline, { values: [42] }));
    const polyline = container.querySelector('polyline');
    expect(polyline).toBeNull();
  });

  it('accepts custom width and height', () => {
    const { container } = render(createElement(Sparkline, { values: [1, 2, 3], width: 120, height: 40 }));
    const svg = container.querySelector('svg');
    expect(svg?.getAttribute('width')).toBe('120');
    expect(svg?.getAttribute('height')).toBe('40');
  });

  it('carries an accessible aria-label when provided', () => {
    render(createElement(Sparkline, { values: [1, 2, 3], ariaLabel: 'tokens/sec over time' }));
    expect(screen.getByLabelText('tokens/sec over time')).toBeTruthy();
  });

  it('renders a flat polyline for two identical values without crashing', () => {
    const { container } = render(createElement(Sparkline, { values: [5, 5], width: 60, height: 20 }));
    const polyline = container.querySelector('polyline');
    expect(polyline).not.toBeNull();
  });
});
