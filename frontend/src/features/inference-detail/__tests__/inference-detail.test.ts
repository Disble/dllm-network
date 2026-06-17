/**
 * Tests for InferenceDetailContainer (container) + InferenceDetailPanel (presentational).
 * Covers view-model mapping for completed events, in-progress honesty (no fabricated metrics),
 * and partial/null state (tokens: null → displays dashes).
 */
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { InferenceDetailContainer } from '../inference-detail-container';
import { InferenceDetailPanel } from '../inference-detail-panel';
import { buildInferenceDetailViewModel } from '../inference-detail-view-model.helpers';

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const createSourceController = (initialSnapshot: DashboardSnapshot) => {
  const listeners = new Set<Parameters<DashboardSnapshotSource['subscribe']>[0]>();
  let currentSnapshot = initialSnapshot;

  return {
    emit(snapshot: DashboardSnapshot) {
      currentSnapshot = snapshot;
      for (const listener of listeners) listener(snapshot);
    },
    source: {
      subscribe(listener: Parameters<DashboardSnapshotSource['subscribe']>[0]) {
        listeners.add(listener);
        return () => { listeners.delete(listener); };
      },
      getSnapshot() { return currentSnapshot; },
    },
  };
};

const makeCompletedEvent = (): InferenceEvent => ({
  at: '2026-06-16T10:00:00Z',
  endpoint: '/api/generate',
  method: 'POST',
  model: 'llama3:8b',
  promptSize: 1024,
  streaming: true,
  status: 1, // PhaseCompleted
  tokens: {
    promptEvalCount: 20,
    evalCount: 80,
    evalDuration: 4000000000,
    totalDuration: 4200000000,
    loadDuration: 100000000,
    perSec: 20.0,
    latencyMS: 4200.0,
  },
});

const makeInProgressEvent = (): InferenceEvent => ({
  at: '2026-06-16T10:01:00Z',
  endpoint: '/api/chat',
  method: 'POST',
  model: 'mistral:7b',
  promptSize: 512,
  streaming: true,
  status: 0, // PhaseInProgress
  tokens: null,
});

const makeSnapshot = (current: InferenceEvent): DashboardSnapshot => ({
  ...EMPTY_DASHBOARD_SNAPSHOT,
  inference: {
    current,
    recent: [current],
  },
});

// ---------------------------------------------------------------------------
// buildInferenceDetailViewModel — pure mapping tests
// ---------------------------------------------------------------------------

describe('buildInferenceDetailViewModel', () => {
  it('maps a completed event to all display fields', () => {
    const event = makeCompletedEvent();
    const vm = buildInferenceDetailViewModel(event);

    expect(vm.model).toBe('llama3:8b');
    expect(vm.endpoint).toBe('/api/generate');
    expect(vm.method).toBe('POST');
    expect(vm.statusLabel).toMatch(/completed/i);
    expect(vm.promptSizeLabel).toMatch(/1/); // 1 KB or 1024 B
    expect(vm.tokenRateLabel).toBe('20.0 tok/s');
    expect(vm.latencyLabel).toBe('4200ms');
    expect(vm.promptEvalCountLabel).toBe('20');
    expect(vm.evalCountLabel).toBe('80');
    expect(vm.timestampLabel).not.toBe('');
  });

  it('does NOT fabricate token metrics for in-progress phase', () => {
    const event = makeInProgressEvent();
    const vm = buildInferenceDetailViewModel(event);

    expect(vm.statusLabel).toMatch(/in.progress/i);
    expect(vm.tokenRateLabel).toBe('—');
    expect(vm.latencyLabel).toBe('—');
    expect(vm.promptEvalCountLabel).toBe('—');
    expect(vm.evalCountLabel).toBe('—');
  });

  it('does NOT fabricate metrics when tokens is null (partial event)', () => {
    const event: InferenceEvent = { ...makeCompletedEvent(), tokens: null };
    const vm = buildInferenceDetailViewModel(event);

    expect(vm.tokenRateLabel).toBe('—');
    expect(vm.latencyLabel).toBe('—');
    expect(vm.promptEvalCountLabel).toBe('—');
    expect(vm.evalCountLabel).toBe('—');
  });

  it('renders promptSizeLabel in KB for sizes >= 1024', () => {
    const event = makeCompletedEvent(); // promptSize: 1024
    const vm = buildInferenceDetailViewModel(event);
    expect(vm.promptSizeLabel).toContain('KB');
  });

  it('renders promptSizeLabel in B for sizes < 1024', () => {
    const event: InferenceEvent = { ...makeCompletedEvent(), promptSize: 512 };
    const vm = buildInferenceDetailViewModel(event);
    expect(vm.promptSizeLabel).toContain('B');
    expect(vm.promptSizeLabel).not.toContain('KB');
  });
});

// ---------------------------------------------------------------------------
// InferenceDetailPanel — presentational component
// ---------------------------------------------------------------------------

describe('InferenceDetailPanel', () => {
  it('renders all completed event fields', () => {
    const event = makeCompletedEvent();
    const vm = buildInferenceDetailViewModel(event);

    render(createElement(InferenceDetailPanel, { viewModel: vm }));

    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('/api/generate')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
    expect(screen.getByText('4200ms')).toBeTruthy();
    expect(screen.getByText('20')).toBeTruthy();
    expect(screen.getByText('80')).toBeTruthy();
  });

  it('renders dashes for metrics when in-progress (no fabrication)', () => {
    const event = makeInProgressEvent();
    const vm = buildInferenceDetailViewModel(event);

    render(createElement(InferenceDetailPanel, { viewModel: vm }));

    // multiple dashes for tok/s, latency, promptEvalCount, evalCount
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it('shows "in progress" status in the panel', () => {
    const event = makeInProgressEvent();
    const vm = buildInferenceDetailViewModel(event);

    render(createElement(InferenceDetailPanel, { viewModel: vm }));

    expect(screen.getByText(/in.progress/i)).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// InferenceDetailContainer — container wired to snapshot source
// ---------------------------------------------------------------------------

describe('InferenceDetailContainer', () => {
  it('renders the detail panel for the current inference event in the snapshot', () => {
    const event = makeCompletedEvent();
    const controller = createSourceController(makeSnapshot(event));

    render(createElement(InferenceDetailContainer, { source: controller.source }));

    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
  });

  it('shows in-progress state honestly when current event is in-progress', () => {
    const event = makeInProgressEvent();
    const controller = createSourceController(makeSnapshot(event));

    render(createElement(InferenceDetailContainer, { source: controller.source }));

    expect(screen.getByText('mistral:7b')).toBeTruthy();
    expect(screen.getByText(/in.progress/i)).toBeTruthy();
    // No tok/s displayed
    expect(screen.queryByText(/tok\/s/)).toBeNull();
  });

  it('renders nothing meaningful when snapshot has empty current event', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(InferenceDetailContainer, { source: controller.source }));

    // should not crash; panel may render empty/no-event state
    expect(screen.queryByText(/tok\/s/)).toBeNull();
  });
});
