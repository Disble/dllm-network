/**
 * Tests for InferenceFeedContainer (container) + InferenceRow (presentational).
 * The container appends live entries as new snapshots arrive WITHOUT full refresh.
 * Container/presentational split: container derives view state; row renders it.
 */
import { createElement } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { InferenceFeedContainer } from '../inference-feed-container';
import { InferenceRow } from '../inference-row';

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// Source controller helper (mirrors dashboard-screen.test.ts pattern)
// ---------------------------------------------------------------------------

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

const makeCompletedEvent = (model: string, perSec: number): InferenceEvent => ({
  at: '2026-06-16T10:00:00Z',
  endpoint: '/api/generate',
  method: 'POST',
  model,
  promptSize: 512,
  streaming: true,
  status: 1, // PhaseCompleted
  tokens: {
    promptEvalCount: 12,
    evalCount: 48,
    evalDuration: 2400000000,
    totalDuration: 2600000000,
    loadDuration: 50000000,
    perSec,
    latencyMS: 2600.0,
  },
});

const makeInProgressEvent = (model: string): InferenceEvent => ({
  at: '2026-06-16T10:01:00Z',
  endpoint: '/api/chat',
  method: 'POST',
  model,
  promptSize: 256,
  streaming: true,
  status: 0, // PhaseInProgress
  tokens: null,
});

const makeSnapshot = (events: InferenceEvent[]): DashboardSnapshot => ({
  ...EMPTY_DASHBOARD_SNAPSHOT,
  inference: {
    current: events[events.length - 1] ?? EMPTY_DASHBOARD_SNAPSHOT.inference.current,
    recent: events,
  },
});

// ---------------------------------------------------------------------------
// InferenceFeedContainer tests
// ---------------------------------------------------------------------------

describe('InferenceFeedContainer', () => {
  it('renders the capture-unavailable banner when no inference events are present', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(InferenceFeedContainer, { source: controller.source }));

    expect(screen.getByText(/capture/i)).toBeTruthy();
  });

  it('renders a live feed row when a completed inference event arrives', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(InferenceFeedContainer, { source: controller.source }));

    act(() => {
      controller.emit(makeSnapshot([makeCompletedEvent('llama3', 20.0)]));
    });

    expect(screen.getByText('llama3')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
  });

  it('appends a new row without removing existing rows on successive snapshots', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(InferenceFeedContainer, { source: controller.source }));

    act(() => {
      controller.emit(makeSnapshot([makeCompletedEvent('llama3', 20.0)]));
    });

    act(() => {
      controller.emit(makeSnapshot([makeCompletedEvent('llama3', 20.0), makeCompletedEvent('mistral', 35.5)]));
    });

    // Both rows should be visible
    expect(screen.getAllByText('llama3')).toHaveLength(1);
    expect(screen.getByText('mistral')).toBeTruthy();
  });

  it('renders in-progress status honestly without fabricated token rate', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(InferenceFeedContainer, { source: controller.source }));

    act(() => {
      controller.emit(makeSnapshot([makeInProgressEvent('gemma3')]));
    });

    expect(screen.getByText('gemma3')).toBeTruthy();
    expect(screen.queryByText(/tok\/s/)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// InferenceRow tests (pure presentational)
// ---------------------------------------------------------------------------

describe('InferenceRow', () => {
  it('renders endpoint, model, and token rate for a completed event', () => {
    const event = makeCompletedEvent('llama3', 20.0);

    render(createElement(InferenceRow, { event }));

    expect(screen.getByText('llama3')).toBeTruthy();
    expect(screen.getByText('/api/generate')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
    expect(screen.getByText('2600ms')).toBeTruthy();
  });

  it('renders em-dash for token rate and latency when event is in-progress', () => {
    const event = makeInProgressEvent('mistral');

    render(createElement(InferenceRow, { event }));

    expect(screen.getByText('mistral')).toBeTruthy();
    // Both TokenRateBadge and LatencyPill should show "—"
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it('renders "in progress" status label for PhaseInProgress', () => {
    const event = makeInProgressEvent('gemma3');

    render(createElement(InferenceRow, { event }));

    expect(screen.getByText(/in.progress/i)).toBeTruthy();
  });

  it('renders "completed" status label for PhaseCompleted', () => {
    const event = makeCompletedEvent('llama3', 15.0);

    render(createElement(InferenceRow, { event }));

    expect(screen.getByText(/completed/i)).toBeTruthy();
  });
});
