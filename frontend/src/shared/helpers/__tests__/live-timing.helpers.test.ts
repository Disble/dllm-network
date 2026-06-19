import { describe, expect, it } from 'vitest';

import type { InferenceEvent } from '../../contracts/dashboard-snapshot.types';
import { PHASE_COMPLETED, PHASE_IN_PROGRESS, PHASE_METADATA_ONLY } from '../../contracts/dashboard-snapshot.types';
import { deriveDisplayTiming } from '../live-timing.helpers';

const baseEvent = (overrides: Partial<InferenceEvent>): InferenceEvent => ({
  at: '2026-06-19T16:42:18Z',
  endpoint: '/v1/chat/completions',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 0,
  streaming: true,
  status: PHASE_IN_PROGRESS,
  tokens: null,
  ...overrides,
});

describe('deriveDisplayTiming', () => {
  it('uses the server/derived token durations once completed (ignores the clock)', () => {
    const event = baseEvent({
      status: PHASE_COMPLETED,
      tokens: {
        promptEvalCount: 1,
        evalCount: 2,
        evalDuration: 4_000_000_000, // 4s in ns
        totalDuration: 5_000_000_000,
        loadDuration: 1_000_000_000, // 1s in ns
        perSec: 0.5,
        latencyMS: 5000,
      },
    });

    const t = deriveDisplayTiming(event, Date.parse('2026-06-19T17:00:00Z'));
    expect(t).toEqual({ loadMS: 1000, evalMS: 4000, totalMS: 5000 });
  });

  it('reports live elapsed wall-clock as the total while in progress (no fabricated phases)', () => {
    const event = baseEvent({ at: '2026-06-19T16:42:18Z', status: PHASE_IN_PROGRESS, tokens: null });
    const now = Date.parse('2026-06-19T16:42:18Z') + 7500; // 7.5s later

    const t = deriveDisplayTiming(event, now);
    expect(t.totalMS).toBe(7500);
    // Load/Eval phases are unknowable until completion — never fabricated.
    expect(t.loadMS).toBeNull();
    expect(t.evalMS).toBeNull();
  });

  it('never returns a negative elapsed (clock skew guard)', () => {
    const event = baseEvent({ at: '2026-06-19T16:42:18Z', status: PHASE_IN_PROGRESS, tokens: null });
    const now = Date.parse('2026-06-19T16:42:18Z') - 1000; // clock went backwards
    expect(deriveDisplayTiming(event, now).totalMS).toBe(0);
  });

  it('returns null total when the in-progress timestamp is unparseable', () => {
    const event = baseEvent({ at: 'not-a-date', status: PHASE_IN_PROGRESS, tokens: null });
    expect(deriveDisplayTiming(event, Date.now()).totalMS).toBeNull();
  });

  it('returns all-null for a metadata-only exchange (no timing applicable)', () => {
    const event = baseEvent({ status: PHASE_METADATA_ONLY, tokens: null });
    expect(deriveDisplayTiming(event, Date.now())).toEqual({ loadMS: null, evalMS: null, totalMS: null });
  });
});
