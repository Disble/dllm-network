/**
 * Tests for TokenRateBadge and LatencyPill atoms.
 * Both must render HONEST unavailable state when data is null.
 */
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

afterEach(() => {
  cleanup();
});

import { TokenRateBadge } from '../token-rate-badge';
import { LatencyPill } from '../latency-pill';

describe('TokenRateBadge', () => {
  it('renders tokens/sec value when available', () => {
    render(createElement(TokenRateBadge, { perSec: 20.5 }));
    expect(screen.getByText('20.5 tok/s')).toBeTruthy();
  });

  it('rounds to one decimal place', () => {
    render(createElement(TokenRateBadge, { perSec: 33.333 }));
    expect(screen.getByText('33.3 tok/s')).toBeTruthy();
  });

  it('renders honest unavailable when perSec is null', () => {
    render(createElement(TokenRateBadge, { perSec: null }));
    expect(screen.getByText('—')).toBeTruthy();
    // Should not render any tok/s text
    expect(screen.queryByText(/tok\/s/)).toBeNull();
  });

  it('carries an aria-label describing the metric', () => {
    render(createElement(TokenRateBadge, { perSec: 15.0 }));
    expect(screen.getByLabelText('15.0 tokens per second')).toBeTruthy();
  });

  it('carries an aria-label indicating unavailable when null', () => {
    render(createElement(TokenRateBadge, { perSec: null }));
    expect(screen.getByLabelText('Token rate unavailable')).toBeTruthy();
  });
});

describe('LatencyPill', () => {
  it('renders latency in ms when available', () => {
    render(createElement(LatencyPill, { latencyMS: 2600.0 }));
    expect(screen.getByText('2600ms')).toBeTruthy();
  });

  it('renders rounded integer ms', () => {
    render(createElement(LatencyPill, { latencyMS: 123.7 }));
    expect(screen.getByText('124ms')).toBeTruthy();
  });

  it('renders honest unavailable when latencyMS is null', () => {
    render(createElement(LatencyPill, { latencyMS: null }));
    expect(screen.getByText('—')).toBeTruthy();
    expect(screen.queryByText(/ms/)).toBeNull();
  });

  it('carries an aria-label describing the latency', () => {
    render(createElement(LatencyPill, { latencyMS: 500.0 }));
    expect(screen.getByLabelText('500ms latency')).toBeTruthy();
  });

  it('carries an aria-label indicating unavailable when null', () => {
    render(createElement(LatencyPill, { latencyMS: null }));
    expect(screen.getByLabelText('Latency unavailable')).toBeTruthy();
  });
});
