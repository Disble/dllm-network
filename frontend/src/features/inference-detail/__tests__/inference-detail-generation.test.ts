/**
 * Tests for the Generation tab: a dedicated, LLM-aware view of the captured
 * /api/generate (or /api/chat) response body. The raw body is JSON — NOT
 * encrypted. The `response` field is a (possibly JSON) string and `context` is
 * an array of tokenizer IDs that we summarise rather than dump.
 *
 * Two layers under test:
 *  - buildGenerationView: pure parser. Honest null when the body is absent or
 *    is not an Ollama generation payload (no fabrication).
 *  - InferenceDetailGeneration: presentational tab wired into the panel.
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import type { InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { InferenceDetailPanel } from '../inference-detail-panel';
import { buildInferenceDetailViewModel } from '../inference-detail-view-model.helpers';
import { buildGenerationView } from '../inference-detail-generation-view-model.helpers';

afterEach(() => {
  cleanup();
});

const OLLAMA_GENERATE_BODY = JSON.stringify({
  model: 'gemma4:12b',
  created_at: '2026-06-18T14:59:35.9829943Z',
  response: '[\n  {"tokens": 1, "ipa": "bʌt"},\n  {"tokens": 1, "ipa": "ˈʌðɚ"}\n]',
  done: true,
  done_reason: 'stop',
  context: [2, 105, 2364, 107, 3048, 659, 614, 7710],
  total_duration: 12872249400,
  eval_count: 142,
});

describe('buildGenerationView', () => {
  it('returns null when the body is absent', () => {
    expect(buildGenerationView(undefined)).toBeNull();
    expect(buildGenerationView('')).toBeNull();
  });

  it('returns null when the body is not an Ollama generation payload', () => {
    expect(buildGenerationView('not json at all')).toBeNull();
    expect(buildGenerationView(JSON.stringify({ models: [] }))).toBeNull();
  });

  it('pretty-prints a JSON `response` and flags it as JSON', () => {
    const view = buildGenerationView(OLLAMA_GENERATE_BODY);
    expect(view).not.toBeNull();
    expect(view?.outputIsJson).toBe(true);
    // Re-indented: nested objects on their own lines, not the escaped one-liner.
    expect(view?.output).toContain('"ipa": "bʌt"');
    expect(view?.output.split('\n').length).toBeGreaterThan(1);
  });

  it('keeps a non-JSON `response` as plain text', () => {
    const body = JSON.stringify({ response: 'just some free text', done_reason: 'stop', context: [1, 2] });
    const view = buildGenerationView(body);
    expect(view?.outputIsJson).toBe(false);
    expect(view?.output).toBe('just some free text');
  });

  it('summarises the context tokens instead of dumping them', () => {
    const view = buildGenerationView(OLLAMA_GENERATE_BODY);
    expect(view?.contextTokenCount).toBe(8);
    expect(view?.contextPreview).toContain('2, 105, 2364');
  });

  it('truncates the context preview with an ellipsis when long', () => {
    const body = JSON.stringify({ response: 'x', context: Array.from({ length: 50 }, (_, i) => i) });
    const view = buildGenerationView(body);
    expect(view?.contextTokenCount).toBe(50);
    expect(view?.contextPreview).toMatch(/…$/);
  });

  it('reports a null context count when context is absent', () => {
    const body = JSON.stringify({ response: 'x', done_reason: 'stop' });
    const view = buildGenerationView(body);
    expect(view).not.toBeNull();
    expect(view?.contextTokenCount).toBeNull();
  });

  it('extracts done_reason', () => {
    expect(buildGenerationView(OLLAMA_GENERATE_BODY)?.doneReason).toBe('stop');
  });
});

const makeGenerationEvent = (): InferenceEvent => ({
  at: '2026-06-18T14:59:35Z',
  endpoint: '/api/generate',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 1024,
  streaming: false,
  status: 1,
  tokens: null,
  responseBody: OLLAMA_GENERATE_BODY,
});

describe('InferenceDetailGeneration tab', () => {
  it('renders a Generation tab that shows parsed output and a context summary', () => {
    const event = makeGenerationEvent();
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Generation' }));
    expect(screen.getByText(/8 tokens/i)).toBeTruthy();
    expect(screen.getByText('stop')).toBeTruthy();
  });

  it('shows an honest not-captured state when the body is absent', () => {
    const event = { ...makeGenerationEvent(), responseBody: undefined };
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Generation' }));
    expect(screen.getByText(/not captured/i)).toBeTruthy();
  });
});
