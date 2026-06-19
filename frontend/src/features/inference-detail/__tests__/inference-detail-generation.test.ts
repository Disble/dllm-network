/**
 * Tests for the Generation tab: a dedicated, LLM-aware view of the model's
 * generated content. The content is NORMALIZED at the backend extractor boundary
 * (internal/telemetry/inference.Generation) — Ollama-native and OpenAI streams
 * both arrive in one shape — so the frontend NEVER parses a response wire format.
 *
 * Two layers under test:
 *  - buildGenerationView: pure presentation mapper over the domain GenerationData
 *    (pretty-print JSON output, summarise the context preview). Honest null when
 *    no generation payload exists — no fabrication.
 *  - InferenceDetailGeneration: presentational tab wired into the panel.
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import type { GenerationData, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { InferenceDetailPanel } from '../inference-detail-panel';
import { buildInferenceDetailViewModel } from '../inference-detail-view-model.helpers';
import { buildGenerationView } from '../inference-detail-generation-view-model.helpers';

afterEach(() => {
  cleanup();
});

const makeGeneration = (overrides: Partial<GenerationData> = {}): GenerationData => ({
  output: 'just some free text',
  reasoning: '',
  finishReason: 'stop',
  contextSize: 0,
  contextPreview: null,
  toolCalls: null,
  ...overrides,
});

describe('buildGenerationView', () => {
  it('returns null when the generation is absent', () => {
    expect(buildGenerationView(undefined)).toBeNull();
    expect(buildGenerationView(null)).toBeNull();
  });

  it('pretty-prints a JSON output and flags it as JSON', () => {
    const view = buildGenerationView(
      makeGeneration({ output: '[\n  {"tokens": 1, "ipa": "bʌt"}\n]' }),
    );
    expect(view).not.toBeNull();
    expect(view?.outputIsJson).toBe(true);
    expect(view?.output).toContain('"ipa": "bʌt"');
    expect(view?.output.split('\n').length).toBeGreaterThan(1);
  });

  it('keeps a non-JSON output as plain text', () => {
    const view = buildGenerationView(makeGeneration({ output: 'just some free text' }));
    expect(view?.outputIsJson).toBe(false);
    expect(view?.output).toBe('just some free text');
  });

  it('exposes the reasoning trace', () => {
    const view = buildGenerationView(makeGeneration({ reasoning: 'first I think, then I answer' }));
    expect(view?.reasoning).toBe('first I think, then I answer');
  });

  it('summarises the context tokens from the bounded preview', () => {
    const view = buildGenerationView(
      makeGeneration({ contextSize: 8, contextPreview: [2, 105, 2364, 107, 3048, 659] }),
    );
    expect(view?.contextTokenCount).toBe(8);
    expect(view?.contextPreview).toContain('2, 105, 2364');
  });

  it('elides the context preview when more tokens exist than the sample', () => {
    const view = buildGenerationView(
      makeGeneration({ contextSize: 50, contextPreview: [0, 1, 2, 3, 4, 5] }),
    );
    expect(view?.contextTokenCount).toBe(50);
    expect(view?.contextPreview).toMatch(/…$/);
  });

  it('reports a null context count when no context is present', () => {
    const view = buildGenerationView(makeGeneration({ contextSize: 0, contextPreview: null }));
    expect(view).not.toBeNull();
    expect(view?.contextTokenCount).toBeNull();
  });

  it('maps finishReason to doneReason, null when empty', () => {
    expect(buildGenerationView(makeGeneration({ finishReason: 'stop' }))?.doneReason).toBe('stop');
    expect(buildGenerationView(makeGeneration({ finishReason: '' }))?.doneReason).toBeNull();
  });

  it('maps tool calls and pretty-prints their JSON arguments', () => {
    const view = buildGenerationView(
      makeGeneration({
        output: '',
        finishReason: 'tool_calls',
        toolCalls: [{ name: 'run_in_terminal', arguments: '{"command":"ls"}' }],
      }),
    );
    expect(view?.toolCalls).toHaveLength(1);
    expect(view?.toolCalls[0].name).toBe('run_in_terminal');
    expect(view?.toolCalls[0].argumentsIsJson).toBe(true);
    expect(view?.toolCalls[0].arguments).toContain('"command": "ls"');
  });

  it('exposes an empty tool-call list when none are present', () => {
    expect(buildGenerationView(makeGeneration({ toolCalls: null }))?.toolCalls).toEqual([]);
  });
});

const makeGenerationEvent = (generation: GenerationData | null = makeGeneration({ contextSize: 8, contextPreview: [2, 105, 2364] })): InferenceEvent => ({
  at: '2026-06-18T14:59:35Z',
  endpoint: '/v1/chat/completions',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 1024,
  streaming: true,
  status: 1,
  tokens: null,
  generation,
});

describe('InferenceDetailGeneration tab', () => {
  it('renders parsed output, reasoning and a context summary', () => {
    const event = makeGenerationEvent(
      makeGeneration({ output: '¡Hola!', reasoning: 'greeting the user', contextSize: 8, contextPreview: [2, 105, 2364] }),
    );
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Generation' }));
    expect(screen.getByText('¡Hola!')).toBeTruthy();
    expect(screen.getByText(/greeting the user/)).toBeTruthy();
    expect(screen.getByText(/8 tokens/i)).toBeTruthy();
    expect(screen.getByText('stop')).toBeTruthy();
  });

  it('renders tool calls (name + arguments) for an agent request with no text', () => {
    const event = makeGenerationEvent(
      makeGeneration({
        output: '',
        finishReason: 'tool_calls',
        toolCalls: [{ name: 'run_in_terminal', arguments: '{"command":"ls -la"}' }],
      }),
    );
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Generation' }));
    expect(screen.getByText('run_in_terminal')).toBeTruthy();
    expect(screen.getByText(/"command": "ls -la"/)).toBeTruthy();
  });

  it('shows an honest not-captured state when no generation exists', () => {
    const event = makeGenerationEvent(null);
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Generation' }));
    expect(screen.getByText(/not captured/i)).toBeTruthy();
  });
});
