/**
 * Tests for the Timing tab's live behavior: while a request is IN PROGRESS the
 * total must tick up with the real elapsed wall-clock (an honest measurement),
 * while the load/eval phases stay "—" because the split is unknown until the
 * request completes (never fabricated).
 */
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import type { InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { PHASE_COMPLETED, PHASE_IN_PROGRESS } from '../../../shared/contracts/dashboard-snapshot.types';
import { InferenceDetailTiming } from '../inference-detail-timing';

afterEach(() => {
  cleanup();
});

const event = (overrides: Partial<InferenceEvent>): InferenceEvent => ({
  at: new Date().toISOString(),
  endpoint: '/v1/chat/completions',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 0,
  streaming: true,
  status: PHASE_IN_PROGRESS,
  tokens: null,
  ...overrides,
});

describe('InferenceDetailTiming live elapsed', () => {
  it('shows a live total while in progress, with load/eval still unavailable', () => {
    // Started 5s ago, still streaming (tokens null).
    const startedAt = new Date(Date.now() - 5000).toISOString();
    render(createElement(InferenceDetailTiming, { event: event({ at: startedAt, status: PHASE_IN_PROGRESS, tokens: null }) }));

    // Total reflects the elapsed wall-clock (~5000ms) — not an em-dash.
    const total = screen.getByText(/^\d+ms$/);
    const totalMS = Number(total.textContent?.replace('ms', ''));
    expect(totalMS).toBeGreaterThanOrEqual(4000);

    // Load and Eval remain honestly unavailable.
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(2);
  });

  it('uses the measured durations once completed', () => {
    render(
      createElement(InferenceDetailTiming, {
        event: event({
          status: PHASE_COMPLETED,
          tokens: {
            promptEvalCount: 1,
            evalCount: 2,
            evalDuration: 4_000_000_000,
            totalDuration: 5_000_000_000,
            loadDuration: 1_000_000_000,
            perSec: 0.5,
            latencyMS: 5000,
          },
        }),
      }),
    );

    expect(screen.getByText('1000ms')).toBeTruthy(); // load
    expect(screen.getByText('4000ms')).toBeTruthy(); // eval
    expect(screen.getByText('5000ms')).toBeTruthy(); // total
  });
});
