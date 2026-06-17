/**
 * Tests for the per-section view-model builder helpers split from createDashboardViewModel.
 * After REFACTOR, buildConfirmedViewModel, buildInferredViewModel, buildPassiveViewModel
 * must exist as named exports and produce correct subsections.
 */
import { describe, expect, it } from 'vitest';

import { buildConfirmedViewModel, buildInferredViewModel, buildPassiveViewModel } from '../dashboard-view-model.helpers';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';

const BASE_SNAPSHOT: DashboardSnapshot = {
  publishedAt: '2026-06-15T00:00:00Z',
  confirmed: {
    ollama: {
      status: 'confirmed',
      reachable: true,
      version: '0.9.0',
      primaryModel: 'llama3',
      runningModels: ['llama3'],
      catalogModelCount: 3,
      observedAt: '2026-06-15T00:00:00Z',
      lastConfirmedAt: '2026-06-15T00:00:00Z',
    },
    system: {
      observedAt: '2026-06-15T00:00:00Z',
      process: { status: 'confirmed', found: true, pid: 1234, cpuPercent: 5.5, rssBytes: 2048 },
      connections: { status: 'confirmed', count: 2 },
      host: { status: 'confirmed', cpuPercent: 15.2, memoryUsedBytes: 8192, memoryTotalBytes: 16384 },
    },
  },
  inferred: {
    current: {
      kind: 'inferred-model-loaded',
      truth: 'inferred',
      model: 'llama3',
      confidence: 'high',
      observedAt: '2026-06-15T00:00:00Z',
      evidence: [{ kind: 'confirmed-running-model', detail: 'confirmed running model: llama3' }],
    },
    recent: [],
  },
  recent: { confirmedModels: [] },
  health: {
    ollama: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-15T00:00:00Z', error: '' },
    process: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-15T00:00:00Z', error: '' },
    connections: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-15T00:00:00Z', error: '' },
    host: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-15T00:00:00Z', error: '' },
  },
  passive: {
    mode: 'passive-only',
    exactRequestLatencyAvailable: false,
    exactTokenCountsAvailable: false,
    exactPayloadAvailable: false,
    exactStatusAvailable: false,
    exactStreamingChunksAvailable: false,
    notes: ['Exact request latency is unavailable in passive mode.'],
  },
  inference: { current: { at: '', endpoint: '', method: '', model: '', promptSize: 0, streaming: false, status: 0, tokens: null }, recent: [] },
};

describe('buildConfirmedViewModel', () => {
  it('extracts primary model and version', () => {
    const result = buildConfirmedViewModel(BASE_SNAPSHOT);
    expect(result.primaryModelValue).toBe('llama3');
    expect(result.ollamaVersionValue).toBe('0.9.0');
  });

  it('falls back to "No confirmed running model" when primaryModel is empty', () => {
    const snapshot: DashboardSnapshot = {
      ...BASE_SNAPSHOT,
      confirmed: {
        ...BASE_SNAPSHOT.confirmed,
        ollama: { ...BASE_SNAPSHOT.confirmed.ollama, primaryModel: '' },
      },
    };
    const result = buildConfirmedViewModel(snapshot);
    expect(result.primaryModelValue).toBe('No confirmed running model');
  });
});

describe('buildInferredViewModel', () => {
  it('extracts inferred summary and confidence', () => {
    const result = buildInferredViewModel(BASE_SNAPSHOT);
    expect(result.inferredSummary).toContain('llama3');
    expect(result.confidenceLabel).toBe('high confidence');
  });
});

describe('buildPassiveViewModel', () => {
  it('extracts passive limitations', () => {
    const result = buildPassiveViewModel(BASE_SNAPSHOT);
    expect(result.passiveLimitations).toContain('Exact request latency is unavailable in passive mode.');
  });
});
