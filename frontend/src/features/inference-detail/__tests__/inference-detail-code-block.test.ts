/**
 * Tests for the shared InferenceDetailCodeBlock: a reusable code viewer with a
 * one-click Pretty/Raw toggle (when a pretty form is available) and a Copy
 * button that always copies the verbatim raw text. Used by Payload, Response and
 * the Generation output so all three behave identically.
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { InferenceDetailCodeBlock } from '../inference-detail-code-block';

const writeText = vi.fn();

beforeEach(() => {
  writeText.mockReset();
  Object.defineProperty(window.navigator, 'clipboard', { value: { writeText }, configurable: true });
});

afterEach(() => {
  cleanup();
});

describe('InferenceDetailCodeBlock', () => {
  it('shows the pretty form by default with a Pretty/Raw toggle when pretty is provided', () => {
    render(createElement(InferenceDetailCodeBlock, { raw: '{"a":1}', pretty: '{\n  "a": 1\n}' }));

    expect(screen.getByRole('button', { name: /pretty/i }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByText(/"a": 1/)).toBeTruthy();
  });

  it('switches to raw on a single click', () => {
    render(createElement(InferenceDetailCodeBlock, { raw: '{"a":1}', pretty: '{\n  "a": 1\n}' }));

    fireEvent.click(screen.getByRole('button', { name: /^raw$/i }));
    expect(screen.getByText('{"a":1}')).toBeTruthy();
  });

  it('omits the toggle when no pretty form is available', () => {
    render(createElement(InferenceDetailCodeBlock, { raw: 'plain text' }));

    expect(screen.queryByRole('button', { name: /pretty/i })).toBeNull();
    expect(screen.getByText('plain text')).toBeTruthy();
  });

  it('always renders a Copy button that copies the verbatim raw text', () => {
    render(createElement(InferenceDetailCodeBlock, { raw: '{"a":1}', pretty: '{\n  "a": 1\n}' }));

    fireEvent.click(screen.getByRole('button', { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith('{"a":1}');
    // Copy works even when the pretty view is showing — it still copies raw.
    expect(screen.getByRole('button', { name: /copied/i })).toBeTruthy();
  });

  it('copies raw text even for a toggle-less (non-JSON) body', () => {
    render(createElement(InferenceDetailCodeBlock, { raw: 'plain text' }));

    fireEvent.click(screen.getByRole('button', { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith('plain text');
  });
});
