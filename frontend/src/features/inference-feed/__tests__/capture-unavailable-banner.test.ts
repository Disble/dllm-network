/**
 * Tests for capture-unavailable banner rendering in InferenceFeedContainer.
 * The banner shows when passive.mode === 'passive-only' (no events yet),
 * using the unelevated note from the backend snapshot.
 */
import { createElement } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { InferenceFeedContainer } from '../inference-feed-container';

afterEach(() => {
  cleanup();
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

describe('InferenceFeedContainer capture-unavailable banner', () => {
  it('shows the capture-unavailable banner when passive-only mode with no events', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);
    render(createElement(InferenceFeedContainer, { source: controller.source }));

    // Banner should be visible
    expect(screen.getByRole('status')).toBeTruthy();
    expect(screen.getByText(/capture/i)).toBeTruthy();
  });

  it('hides the banner when inference events arrive from capture-active snapshot', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);
    render(createElement(InferenceFeedContainer, { source: controller.source }));

    // Initially shows banner
    expect(screen.getByRole('status')).toBeTruthy();

    act(() => {
      controller.emit({
        ...EMPTY_DASHBOARD_SNAPSHOT,
        passive: {
          mode: 'capture-active',
          exactRequestLatencyAvailable: true,
          exactTokenCountsAvailable: true,
          exactPayloadAvailable: true,
          exactStatusAvailable: true,
          exactStreamingChunksAvailable: true,
          notes: [],
        },
        inference: {
          current: {
            at: '2026-06-16T10:00:00Z',
            endpoint: '/api/generate',
            method: 'POST',
            model: 'llama3',
            promptSize: 512,
            streaming: true,
            status: 1,
            tokens: { promptEvalCount: 12, evalCount: 48, evalDuration: 2400000000, totalDuration: 2600000000, loadDuration: 50000000, perSec: 20.0, latencyMS: 2600.0 },
          },
          recent: [{
            at: '2026-06-16T10:00:00Z',
            endpoint: '/api/generate',
            method: 'POST',
            model: 'llama3',
            promptSize: 512,
            streaming: true,
            status: 1,
            tokens: { promptEvalCount: 12, evalCount: 48, evalDuration: 2400000000, totalDuration: 2600000000, loadDuration: 50000000, perSec: 20.0, latencyMS: 2600.0 },
          }],
        },
      });
    });

    // Banner should be gone, inference row should be present
    expect(screen.queryByRole('status')).toBeNull();
    expect(screen.getByText('llama3')).toBeTruthy();
  });

  it('includes the unelevated note from backend passive.notes when in passive-only mode', () => {
    const snapshotWithNote: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      passive: {
        ...EMPTY_DASHBOARD_SNAPSHOT.passive,
        notes: [
          ...EMPTY_DASHBOARD_SNAPSHOT.passive.notes,
          'run as administrator to enable live capture',
        ],
      },
    };
    const controller = createSourceController(snapshotWithNote);
    render(createElement(InferenceFeedContainer, { source: controller.source }));

    expect(screen.getByText(/administrator/i)).toBeTruthy();
  });
});
