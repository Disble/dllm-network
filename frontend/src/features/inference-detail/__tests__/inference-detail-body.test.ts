/**
 * Tests for the shared InferenceDetailBody renderer used by the Payload and
 * Response tabs. When the captured body is JSON, the renderer offers a one-click
 * Pretty/Raw toggle (Pretty by default). Non-JSON bodies render verbatim with no
 * toggle. Absent bodies render the honest not-captured notice.
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { InferenceDetailBody } from '../inference-detail-body';
import { formatJsonPretty } from '../inference-detail-body.helpers';

afterEach(() => {
  cleanup();
});

describe('formatJsonPretty', () => {
  it('returns indented JSON for a valid JSON string', () => {
    const pretty = formatJsonPretty('{"a":1,"b":[2,3]}');
    expect(pretty).not.toBeNull();
    expect(pretty?.split('\n').length).toBeGreaterThan(1);
    expect(pretty).toContain('"a": 1');
  });

  it('returns null for non-JSON text', () => {
    expect(formatJsonPretty('hello world')).toBeNull();
  });

  it('returns null for a bare JSON scalar (nothing to pretty-print)', () => {
    expect(formatJsonPretty('42')).toBeNull();
    expect(formatJsonPretty('"just a string"')).toBeNull();
  });
});

describe('InferenceDetailBody', () => {
  it('renders the not-captured notice when the body is absent', () => {
    render(createElement(InferenceDetailBody, { body: undefined }));
    expect(screen.getByText(/not captured/i)).toBeTruthy();
  });

  it('pretty-prints JSON by default and exposes a Pretty/Raw toggle', () => {
    render(createElement(InferenceDetailBody, { body: '{"a":1,"b":2}' }));

    expect(screen.getByRole('button', { name: /pretty/i })).toBeTruthy();
    expect(screen.getByRole('button', { name: /raw/i })).toBeTruthy();
    // Pretty is the default selection.
    expect(screen.getByRole('button', { name: /pretty/i }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByText(/"a": 1/)).toBeTruthy();
  });

  it('switches to the verbatim raw body on a single click', () => {
    const raw = '{"a":1,"b":2}';
    render(createElement(InferenceDetailBody, { body: raw }));

    fireEvent.click(screen.getByRole('button', { name: /raw/i }));
    expect(screen.getByRole('button', { name: /raw/i }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByText(raw)).toBeTruthy();
  });

  it('renders non-JSON bodies verbatim with no toggle', () => {
    render(createElement(InferenceDetailBody, { body: 'plain text body' }));

    expect(screen.queryByRole('button', { name: /pretty/i })).toBeNull();
    expect(screen.getByText('plain text body')).toBeTruthy();
  });

  it('still surfaces the truncation notice', () => {
    render(createElement(InferenceDetailBody, { body: 'plain text', truncated: true }));
    expect(screen.getByText(/truncated/i)).toBeTruthy();
  });
});
