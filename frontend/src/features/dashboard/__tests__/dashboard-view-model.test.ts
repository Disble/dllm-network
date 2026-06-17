import { describe, expect, it } from 'vitest';

import { createDashboardViewModel } from '../dashboard-view-model.helpers';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';

describe('createDashboardViewModel', () => {
  it('labels confirmed and inferred telemetry and exposes passive limits', () => {
    const snapshot = createSnapshot();

    const viewModel = createDashboardViewModel(snapshot, new Date('2026-06-15T00:00:30Z'));

    expect(viewModel.confirmedBadgeLabel).toBe('Confirmed telemetry');
    expect(viewModel.inferredBadgeLabel).toBe('Inferred activity');
    expect(viewModel.passiveLimitations).toEqual([
      'Exact request latency is unavailable in passive mode.',
      'Exact token counts are unavailable in passive mode.',
      'Exact request and response payloads are unavailable in passive mode.',
      'Exact HTTP status results are unavailable in passive mode.',
      'Exact streaming chunks are unavailable in passive mode.',
    ]);
    expect(viewModel.primaryModelValue).toBe('mistral');
    expect(viewModel.stalenessLabel).toBe('Fresh passive snapshot');
    expect(viewModel.evidence).toEqual(['confirmed running model: mistral', 'owned loopback connection activity is present']);
  });

  it('marks stale snapshots when the latest passive signal ages out', () => {
    const snapshot = createSnapshot({ publishedAt: '2026-06-15T00:00:00Z' });

    const viewModel = createDashboardViewModel(snapshot, new Date('2026-06-15T00:03:30Z'));

    expect(viewModel.stalenessLabel).toBe('Stale passive snapshot');
  });
});

function createSnapshot(overrides?: Partial<DashboardSnapshot>): DashboardSnapshot {
  return {
    publishedAt: overrides?.publishedAt ?? '2026-06-15T00:00:00Z',
    confirmed: overrides?.confirmed ?? {
      ollama: {
        status: 'confirmed',
        reachable: true,
        version: '0.8.0',
        primaryModel: 'mistral',
        runningModels: ['mistral'],
        catalogModelCount: 2,
        observedAt: '2026-06-14T23:59:55Z',
        lastConfirmedAt: '2026-06-14T23:59:55Z',
      },
      system: {
        observedAt: '2026-06-14T23:59:57Z',
        process: {
          status: 'confirmed',
          found: true,
          pid: 4242,
          cpuPercent: 12.3,
          rssBytes: 1024,
        },
        connections: {
          status: 'confirmed',
          count: 1,
        },
        host: {
          status: 'confirmed',
          cpuPercent: 20.4,
          memoryUsedBytes: 4096,
          memoryTotalBytes: 8192,
        },
      },
    },
    inferred: overrides?.inferred ?? {
      current: {
        kind: 'inferred-model-loaded',
        truth: 'inferred',
        model: 'mistral',
        confidence: 'high',
        observedAt: '2026-06-14T23:59:59Z',
        evidence: [
          { kind: 'confirmed-running-model', detail: 'confirmed running model: mistral' },
          { kind: 'confirmed-connection-activity-present', detail: 'owned loopback connection activity is present' },
        ],
      },
      recent: [],
    },
    recent: overrides?.recent ?? {
      confirmedModels: [{ observedAt: '2026-06-14T23:58:00Z', model: 'gemma3' }],
    },
    health: overrides?.health ?? {
      ollama: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:55Z', error: '' },
      process: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
      connections: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
      host: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
    },
    passive: overrides?.passive ?? {
      mode: 'passive-only',
      exactRequestLatencyAvailable: false,
      exactTokenCountsAvailable: false,
      exactPayloadAvailable: false,
      exactStatusAvailable: false,
      exactStreamingChunksAvailable: false,
      notes: [
        'Exact request latency is unavailable in passive mode.',
        'Exact token counts are unavailable in passive mode.',
        'Exact request and response payloads are unavailable in passive mode.',
        'Exact HTTP status results are unavailable in passive mode.',
        'Exact streaming chunks are unavailable in passive mode.',
      ],
    },
    inference: overrides?.inference ?? { current: { at: '', endpoint: '', method: '', model: '', promptSize: 0, streaming: false, status: 0, tokens: null }, recent: [] },
  };
}
