/**
 * Tests for the top KPI strip (InferenceKpiContainer + useInferenceMetrics):
 * it derives request count, tok/s, latency percentiles, eval count and the
 * last-updated timestamp from the shared store.
 */
import { createElement } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import {
  PHASE_COMPLETED,
  PHASE_IN_PROGRESS,
} from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { resetInferenceStore, useInferenceStore } from '../../../shared/store/inference-store';
import { InferenceKpiContainer } from '../inference-kpi-container';

beforeEach(() => {
  resetInferenceStore();
});

afterEach(() => {
  cleanup();
  resetInferenceStore();
});

function makeSource(snapshot: DashboardSnapshot): DashboardSnapshotSource {
  return {
    subscribe() {
      return () => {};
    },
    getSnapshot: () => snapshot,
  };
}

const completedEvent: InferenceEvent = {
  at: '2026-06-18T14:23:02Z',
  endpoint: '/api/generate',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 1400,
  streaming: true,
  status: PHASE_COMPLETED,
  tokens: {
    promptEvalCount: 386,
    evalCount: 169,
    evalDuration: 0,
    totalDuration: 0,
    loadDuration: 0,
    perSec: 45,
    latencyMS: 16757,
  },
};

const secondCompletedEvent: InferenceEvent = {
  ...completedEvent,
  id: 'second-completed',
  at: '2026-06-18T14:25:02Z',
  endpoint: '/api/chat',
  model: 'llama3',
  tokens: {
    ...completedEvent.tokens!,
    evalCount: 50,
    perSec: 30,
    latencyMS: 4000,
  },
};

const inProgressEvent: InferenceEvent = {
  ...completedEvent,
  id: 'third-progress',
  at: '2026-06-18T14:27:02Z',
  endpoint: '/api/generate',
  model: 'llama3',
  status: PHASE_IN_PROGRESS,
  tokens: null,
};

describe('InferenceKpiContainer', () => {
  it('renders all six KPI labels', () => {
    render(createElement(InferenceKpiContainer, { source: makeSource(EMPTY_DASHBOARD_SNAPSHOT) }));
    for (const label of ['Requests', 'Avg tok/s', 'P50 latency', 'P95 latency', 'Eval count', 'Timestamp']) {
      expect(screen.getByText(label)).toBeTruthy();
    }
  });

  it('shows honest em-dashes when no completed events exist', () => {
    render(createElement(InferenceKpiContainer, { source: makeSource(EMPTY_DASHBOARD_SNAPSHOT) }));
    expect(screen.getAllByText('0').length).toBeGreaterThanOrEqual(1); // Requests + Eval count
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(3); // avg/p50/p95/timestamp
  });

  it('computes values from captured events', () => {
    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      inference: { current: EMPTY_DASHBOARD_SNAPSHOT.inference.current, recent: [completedEvent] },
    };
    render(createElement(InferenceKpiContainer, { source: makeSource(snapshot) }));
    expect(screen.getByText('45.0 tok/s')).toBeTruthy();
    expect(screen.getByText('169')).toBeTruthy(); // eval count total
  });

  it('tracks active query and status filters from the request table store', () => {
    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      inference: {
        current: EMPTY_DASHBOARD_SNAPSHOT.inference.current,
        recent: [completedEvent, secondCompletedEvent, inProgressEvent],
      },
    };

    render(createElement(InferenceKpiContainer, { source: makeSource(snapshot) }));

    expect(screen.getByText('3')).toBeTruthy();
    expect(screen.getByText('37.5 tok/s')).toBeTruthy();
    expect(screen.getByText('219')).toBeTruthy();
    expect(screen.getByText('2026-06-18 14:27:02Z')).toBeTruthy();

    act(() => {
      useInferenceStore.getState().setQuery('llama3');
      useInferenceStore.getState().setStatusFilter(PHASE_COMPLETED);
    });

    expect(screen.getByText('1')).toBeTruthy();
    expect(screen.getByText('30.0 tok/s')).toBeTruthy();
    expect(screen.getByText('50')).toBeTruthy();
    expect(screen.getByText('2026-06-18 14:25:02Z')).toBeTruthy();

    act(() => {
      useInferenceStore.getState().setQuery('');
      useInferenceStore.getState().setStatusFilter('all');
    });

    expect(screen.getByText('3')).toBeTruthy();
    expect(screen.getByText('37.5 tok/s')).toBeTruthy();
    expect(screen.getByText('219')).toBeTruthy();
    expect(screen.getByText('2026-06-18 14:27:02Z')).toBeTruthy();
  });
});
