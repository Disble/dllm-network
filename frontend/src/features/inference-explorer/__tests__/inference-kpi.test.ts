/**
 * Tests for the top KPI strip (InferenceKpiContainer + useInferenceMetrics):
 * it derives request count, tok/s, latency percentiles, eval count and the
 * last-updated timestamp from the shared store.
 */
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { PHASE_COMPLETED } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { resetInferenceStore } from '../../../shared/store/inference-store';
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
});
