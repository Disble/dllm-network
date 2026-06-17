/**
 * Tests that DashboardSnapshot carries InferenceState with correct shape
 * and that EMPTY_DASHBOARD_SNAPSHOT provides safe zero-value defaults.
 */
import { describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceState, InferenceEvent, RunningModelView } from '../dashboard-snapshot.types';

describe('DashboardSnapshot inference contract', () => {
  it('EMPTY_DASHBOARD_SNAPSHOT has a zero-value inference state', () => {
    const snapshot: DashboardSnapshot = EMPTY_DASHBOARD_SNAPSHOT;

    expect(snapshot.inference).toBeDefined();
    expect(snapshot.inference.current).toBeDefined();
    expect(snapshot.inference.recent).toEqual([]);
    // current should be zero-value: no tokens, status=PhaseInProgress(0)
    expect(snapshot.inference.current.tokens).toBeNull();
    expect(snapshot.inference.current.status).toBe(0);
    expect(snapshot.inference.current.model).toBe('');
  });

  it('InferenceState with a completed inference carries token stats', () => {
    const event: InferenceEvent = {
      at: '2026-06-16T10:00:00Z',
      endpoint: '/api/generate',
      method: 'POST',
      model: 'llama3',
      promptSize: 512,
      streaming: true,
      status: 1, // PhaseCompleted
      tokens: {
        promptEvalCount: 12,
        evalCount: 48,
        evalDuration: 2400000000,
        totalDuration: 2600000000,
        loadDuration: 50000000,
        perSec: 20.0,
        latencyMS: 2600.0,
      },
    };

    const inferenceState: InferenceState = {
      current: event,
      recent: [event],
    };

    expect(inferenceState.current.tokens).not.toBeNull();
    expect(inferenceState.current.tokens?.perSec).toBe(20.0);
    expect(inferenceState.current.tokens?.latencyMS).toBe(2600.0);
    expect(inferenceState.current.tokens?.evalCount).toBe(48);
    expect(inferenceState.recent).toHaveLength(1);
  });

  it('InferenceEvent with PhaseInProgress has null tokens (honest unavailability)', () => {
    const event: InferenceEvent = {
      at: '2026-06-16T10:01:00Z',
      endpoint: '/api/chat',
      method: 'POST',
      model: 'mistral',
      promptSize: 256,
      streaming: true,
      status: 0, // PhaseInProgress
      tokens: null,
    };

    expect(event.tokens).toBeNull();
  });

  it('DashboardSnapshot confirmed.ollama carries runningModelDetails alongside runningModels', () => {
    const detail: RunningModelView = {
      name: 'llama3:8b',
      size: 4500000000,
      sizeVram: 4200000000,
      parameterSize: '8B',
      quantizationLevel: 'Q4_0',
      contextLength: 8192,
      expiresAt: '2026-06-16T13:00:00Z',
    };

    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      confirmed: {
        ...EMPTY_DASHBOARD_SNAPSHOT.confirmed,
        ollama: {
          ...EMPTY_DASHBOARD_SNAPSHOT.confirmed.ollama,
          runningModels: ['llama3:8b'],
          runningModelDetails: [detail],
        },
      },
    };

    expect(snapshot.confirmed.ollama.runningModelDetails).toHaveLength(1);
    expect(snapshot.confirmed.ollama.runningModelDetails![0].sizeVram).toBe(4200000000);
    expect(snapshot.confirmed.ollama.runningModelDetails![0].parameterSize).toBe('8B');
  });

  it('EMPTY_DASHBOARD_SNAPSHOT.confirmed.ollama has empty runningModelDetails by default', () => {
    expect(EMPTY_DASHBOARD_SNAPSHOT.confirmed.ollama.runningModelDetails).toEqual([]);
  });

  it('EMPTY_DASHBOARD_SNAPSHOT.passive has captureMode field compatible with updated type', () => {
    expect(EMPTY_DASHBOARD_SNAPSHOT.passive.mode).toBe('passive-only');
    expect(EMPTY_DASHBOARD_SNAPSHOT.passive.notes).toBeInstanceOf(Array);
  });
});
