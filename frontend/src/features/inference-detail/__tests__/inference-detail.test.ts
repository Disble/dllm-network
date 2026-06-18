/**
 * Tests for the tabbed InferenceDetailPanel (presentational) + InferenceDetailContainer
 * (reads the SELECTED event from the shared store). Covers Overview view-model
 * mapping, in-progress honesty (no fabricated metrics), the not-captured state
 * for backend-pending tabs, and master-detail selection.
 */
import { createElement } from 'react';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { deriveEventId } from '../../../shared/store/inference-store.helpers';
import { resetInferenceStore, useInferenceStore } from '../../../shared/store/inference-store';
import { InferenceDetailContainer } from '../inference-detail-container';
import { InferenceDetailPanel } from '../inference-detail-panel';
import { buildInferenceDetailViewModel } from '../inference-detail-view-model.helpers';

beforeEach(() => {
  resetInferenceStore();
});

afterEach(() => {
  cleanup();
  resetInferenceStore();
});

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
  status: 1,
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
  status: 0,
  tokens: null,
});

const makeSnapshot = (current: InferenceEvent): DashboardSnapshot => ({
  ...EMPTY_DASHBOARD_SNAPSHOT,
  inference: { current, recent: [current] },
});

describe('buildInferenceDetailViewModel', () => {
  it('maps a completed event to all display fields', () => {
    const vm = buildInferenceDetailViewModel(makeCompletedEvent());
    expect(vm.model).toBe('llama3:8b');
    expect(vm.endpoint).toBe('/api/generate');
    expect(vm.statusLabel).toMatch(/completed/i);
    expect(vm.tokenRateLabel).toBe('20.0 tok/s');
    expect(vm.latencyLabel).toBe('4200ms');
    expect(vm.promptEvalCountLabel).toBe('20');
    expect(vm.evalCountLabel).toBe('80');
  });

  it('does NOT fabricate token metrics for in-progress phase', () => {
    const vm = buildInferenceDetailViewModel(makeInProgressEvent());
    expect(vm.statusLabel).toMatch(/in.progress/i);
    expect(vm.tokenRateLabel).toBe('—');
    expect(vm.latencyLabel).toBe('—');
  });
});

describe('InferenceDetailPanel', () => {
  it('prompts to select a request when no event is selected', () => {
    render(createElement(InferenceDetailPanel, { event: null, overview: null }));
    expect(screen.getByText(/select a request/i)).toBeTruthy();
  });

  it('renders completed event fields in the default Overview tab', () => {
    const event = makeCompletedEvent();
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
    expect(screen.getByText('4200ms')).toBeTruthy();
  });

  it('shows an honest not-captured state on the Payload tab', () => {
    const event = makeCompletedEvent();
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Payload' }));
    expect(screen.getByText(/not captured/i)).toBeTruthy();
  });

  it('renders real timing on the Timing tab for completed events', () => {
    const event = makeCompletedEvent();
    const overview = buildInferenceDetailViewModel(event);
    render(createElement(InferenceDetailPanel, { event, overview }));

    fireEvent.click(screen.getByRole('tab', { name: 'Timing' }));
    expect(screen.getByText('Load')).toBeTruthy();
    expect(screen.getByText('Eval')).toBeTruthy();
  });
});

describe('InferenceDetailContainer', () => {
  it('renders the detail for the SELECTED event (not just the latest)', () => {
    const event = makeCompletedEvent();
    const controller = createSourceController(makeSnapshot(event));

    render(createElement(InferenceDetailContainer, { source: controller.source }));
    act(() => { useInferenceStore.getState().select(deriveEventId(event)); });

    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('20.0 tok/s')).toBeTruthy();
  });

  it('prompts to select when nothing is selected', () => {
    const controller = createSourceController(makeSnapshot(makeCompletedEvent()));
    render(createElement(InferenceDetailContainer, { source: controller.source }));

    expect(screen.getByText(/select a request/i)).toBeTruthy();
    expect(screen.queryByText(/tok\/s/)).toBeNull();
  });
});
