import { createElement } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';
import { DashboardScreen } from '../dashboard-screen';

afterEach(() => {
  cleanup();
});

describe('DashboardScreen', () => {
  it('prioritizes the inference network workbench before telemetry context', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(DashboardScreen, { source: controller.source, now: new Date('2026-06-15T00:03:30Z') }));

    const workbench = screen.getByLabelText('Inference network');
    const telemetryPanel = screen.getByText('Passive-only telemetry').closest('.dashboard-shell');
    const secondaryWorkspace = screen.getByLabelText('Secondary telemetry');

    expect(telemetryPanel).not.toBeNull();
    expect(secondaryWorkspace.contains(telemetryPanel)).toBe(true);
    expect(secondaryWorkspace.contains(screen.getByLabelText('Running models'))).toBe(true);
    expect(workbench.compareDocumentPosition(telemetryPanel!) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it('renders the passive-safe empty view before the first dashboard snapshot event', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(DashboardScreen, { source: controller.source, now: new Date('2026-06-15T00:03:30Z') }));

    expect(screen.getByText('No confirmed running model')).toBeTruthy();
    expect(screen.getByText('Stale passive snapshot')).toBeTruthy();
    expect(screen.getByText('No confirmed model history yet.')).toBeTruthy();
    expect(screen.getByText(hasExactText('Published Unavailable'))).toBeTruthy();
    expect(screen.getByText(hasExactText('Ollama version: Unavailable'))).toBeTruthy();
    expect(screen.getByText(hasExactText('Observed: Unavailable'))).toBeTruthy();
  });

  it('re-renders when a dashboard:snapshot event delivers a live passive snapshot', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(DashboardScreen, { source: controller.source, now: new Date('2026-06-15T00:00:30Z') }));

    expect(screen.getByText('No confirmed running model')).toBeTruthy();

    act(() => {
      controller.emit(createSnapshot('mistral', 'inferred-model-changed'));
    });

    expect(screen.getByText('mistral')).toBeTruthy();
    expect(screen.getByText('Fresh passive snapshot')).toBeTruthy();
    expect(screen.getByText('inferred-model-changed • mistral')).toBeTruthy();
    expect(screen.getByText('confirmed running model: mistral')).toBeTruthy();
    expect(screen.queryByText('No confirmed running model')).toBeNull();
  });
});

const createSourceController = (initialSnapshot: DashboardSnapshot) => {
  const listeners = new Set<Parameters<DashboardSnapshotSource['subscribe']>[0]>();
  let currentSnapshot = initialSnapshot;

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
  inference: { current: { at: '', endpoint: '', method: '', model: '', promptSize: 0, streaming: false, status: 0, tokens: null }, recent: [] },
});

const hasExactText = (expected: string) => (_content: string, element: { textContent: string | null } | null) =>
  element?.textContent === expected;
