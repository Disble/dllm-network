/**
 * Tests for InferenceExplorerContainer — focused on the capture-unavailable
 * banner ported from the retired inference-feed feature. Virtualized row
 * rendering is covered indirectly (jsdom has no layout); these assert the
 * banner gate and the toolbar render.
 */
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { PHASE_COMPLETED } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { resetInferenceStore } from '../../../shared/store/inference-store';
import { InferenceExplorerContainer } from '../inference-explorer-container';

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
  at: '2026-06-18T03:17:18Z',
  endpoint: '/api/generate',
  method: 'POST',
  model: 'gemma4:12b',
  promptSize: 1300,
  streaming: true,
  status: PHASE_COMPLETED,
  tokens: null,
};

describe('InferenceExplorerContainer capture-unavailable banner', () => {
  it('shows the banner when no events and passive-only mode', () => {
    render(createElement(InferenceExplorerContainer, { source: makeSource(EMPTY_DASHBOARD_SNAPSHOT) }));
    expect(screen.getByText(/live inference capture is unavailable/i)).toBeTruthy();
  });

  it('hides the banner once events have been captured', () => {
    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      inference: { current: EMPTY_DASHBOARD_SNAPSHOT.inference.current, recent: [completedEvent] },
    };
    render(createElement(InferenceExplorerContainer, { source: makeSource(snapshot) }));
    expect(screen.queryByText(/live inference capture is unavailable/i)).toBeNull();
  });

  it('hides the banner when capture is active', () => {
    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      passive: { ...EMPTY_DASHBOARD_SNAPSHOT.passive, mode: 'capture-active' },
    };
    render(createElement(InferenceExplorerContainer, { source: makeSource(snapshot) }));
    expect(screen.queryByText(/live inference capture is unavailable/i)).toBeNull();
  });
});
