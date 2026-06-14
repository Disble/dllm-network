import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';
import { useDashboardSnapshot } from '../use-dashboard-snapshot';

describe('useDashboardSnapshot', () => {
  it('subscribes to the source and updates when a new passive snapshot arrives', () => {
    const controller = createSourceController();
    const initialSnapshot = createSnapshot('gemma3', 'inferred-idle');
    const { result } = renderHook(() =>
      useDashboardSnapshot({
        source: controller.source,
        initialSnapshot,
      }),
    );

    expect(result.current.confirmed.ollama.primaryModel).toBe('gemma3');

    act(() => {
      controller.emit(createSnapshot('mistral', 'inferred-model-changed'));
    });

    expect(result.current.confirmed.ollama.primaryModel).toBe('mistral');
    expect(result.current.inferred.current.kind).toBe('inferred-model-changed');
  });
});

const createSourceController = () => {
  const listeners = new Set<Parameters<DashboardSnapshotSource['subscribe']>[0]>();
  let currentSnapshot = createSnapshot('gemma3', 'inferred-idle');

  return {
    emit(snapshot: DashboardSnapshot) {
      currentSnapshot = snapshot;
      for (const listener of listeners) {
        listener(snapshot);
      }
    },
    source: {
      subscribe(listener: Parameters<DashboardSnapshotSource['subscribe']>[0]) {
        listeners.add(listener);

        return () => {
          listeners.delete(listener);
        };
      },
      getSnapshot() {
        return currentSnapshot;
      },
    },
  };
};

const createSnapshot = (model: string, kind: DashboardSnapshot['inferred']['current']['kind']): DashboardSnapshot => ({
  publishedAt: '2026-06-15T00:00:00Z',
  confirmed: {
    ollama: {
      status: 'confirmed',
      reachable: true,
      version: '0.8.0',
      primaryModel: model,
      runningModels: [model],
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
  inferred: {
    current: {
      kind,
      truth: 'inferred',
      model,
      confidence: 'high',
      observedAt: '2026-06-14T23:59:59Z',
      evidence: [{ kind: 'confirmed-running-model', detail: `confirmed running model: ${model}` }],
    },
    recent: [],
  },
  recent: {
    confirmedModels: [{ observedAt: '2026-06-14T23:58:00Z', model }],
  },
  health: {
    ollama: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:55Z', error: '' },
    process: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
    connections: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
    host: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
  },
  passive: {
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
});
