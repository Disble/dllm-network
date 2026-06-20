import { describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';
import { createDashboardViewModel, humaniseCollectionMode } from '../dashboard-view-model.helpers';

function snapshotAt(publishedAt: string, healthy: boolean): DashboardSnapshot {
  return {
    ...EMPTY_DASHBOARD_SNAPSHOT,
    publishedAt,
    health: {
      ...EMPTY_DASHBOARD_SNAPSHOT.health,
      ollama: { ...EMPTY_DASHBOARD_SNAPSHOT.health.ollama, healthy },
    },
  };
}

describe('createDashboardViewModel', () => {
  it('summarises a fresh, healthy passive snapshot', () => {
    const vm = createDashboardViewModel(snapshotAt('2026-06-15T00:00:00Z', true), new Date('2026-06-15T00:00:30Z'));

    expect(vm.isFresh).toBe(true);
    expect(vm.stalenessLabel).toBe('Fresh passive snapshot');
    expect(vm.collectionModeLabel).toBe('Passive-only');
    expect(vm.healthLabel).toBe('Healthy');
    expect(vm.snapshotTimeLabel).toBe('2026-06-14 19:00:00 -05:00');
  });

  it('marks stale snapshots when the signal ages out', () => {
    const vm = createDashboardViewModel(snapshotAt('2026-06-15T00:00:00Z', true), new Date('2026-06-15T00:03:30Z'));

    expect(vm.isFresh).toBe(false);
    expect(vm.stalenessLabel).toBe('Stale passive snapshot');
  });

  it('reports unavailable health honestly', () => {
    const vm = createDashboardViewModel(snapshotAt('2026-06-15T00:00:00Z', false), new Date('2026-06-15T00:00:30Z'));

    expect(vm.healthLabel).toBe('Unavailable');
  });
});

describe('humaniseCollectionMode', () => {
  it('humanises known modes and falls back gracefully', () => {
    expect(humaniseCollectionMode('passive-only')).toBe('Passive-only');
    expect(humaniseCollectionMode('capture-active')).toBe('Capture active');
    expect(humaniseCollectionMode('')).toBe('Unknown');
  });
});
