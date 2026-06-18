/**
 * Tests for InferenceDetailContextToggle: a small disclosure control (count
 * label + rotating chevron) extracted from the Generation tab. Replaces the raw
 * unicode "⌄" glyph with a centered SVG chevron. Presentational: open state and
 * the toggle handler are owned by the parent.
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { InferenceDetailContextToggle } from '../inference-detail-context-toggle';

afterEach(() => {
  cleanup();
});

describe('InferenceDetailContextToggle', () => {
  it('renders the label inside a toggle button', () => {
    render(createElement(InferenceDetailContextToggle, { label: '521 tokens', open: false, onToggle: vi.fn() }));

    const button = screen.getByRole('button', { name: /521 tokens/i });
    expect(button).toBeTruthy();
    expect(button.getAttribute('aria-expanded')).toBe('false');
  });

  it('reflects the open state via aria-expanded', () => {
    render(createElement(InferenceDetailContextToggle, { label: '521 tokens', open: true, onToggle: vi.fn() }));
    expect(screen.getByRole('button', { name: /521 tokens/i }).getAttribute('aria-expanded')).toBe('true');
  });

  it('renders an SVG chevron (not a raw unicode glyph)', () => {
    const { container } = render(
      createElement(InferenceDetailContextToggle, { label: '521 tokens', open: false, onToggle: vi.fn() }),
    );
    expect(container.querySelector('svg')).toBeTruthy();
    expect(container.textContent).not.toContain('⌄');
    expect(container.textContent).not.toContain('⌃');
  });

  it('calls onToggle when clicked', () => {
    const onToggle = vi.fn();
    render(createElement(InferenceDetailContextToggle, { label: '521 tokens', open: false, onToggle }));

    fireEvent.click(screen.getByRole('button', { name: /521 tokens/i }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
