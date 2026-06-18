import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../contracts/dashboard-snapshot.constants';
import type {
  DashboardSnapshot,
  InferenceEvent,
} from '../../contracts/dashboard-snapshot.types';
import { PHASE_COMPLETED, PHASE_IN_PROGRESS } from '../../contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import {
  computeAggregates,
  deriveEventId,
  ingestSnapshotEvents,
  selectEventById,
  selectFilteredEvents,
} from '../inference-store.helpers';
import {
  connectInferenceStore,
  resetInferenceStore,
  useInferenceStore,
} from '../inference-store';

function makeEvent(overrides: Partial<InferenceEvent>): InferenceEvent {
  return {
    at: '2026-06-18T03:17:18.893Z',
    endpoint: '/api/generate',
    method: 'POST',
    model: 'gemma4:12b',
    promptSize: 1300,
    streaming: true,
    status: PHASE_COMPLETED,
    tokens: {
      promptEvalCount: 341,
      evalCount: 64,
      evalDuration: 0,
      totalDuration: 0,
      loadDuration: 0,
      perSec: 46.7,
      latencyMS: 2616,
    },
    ...overrides,
  };
}

function snapshotWith(recent: readonly InferenceEvent[], current?: InferenceEvent): DashboardSnapshot {
  return {
    ...EMPTY_DASHBOARD_SNAPSHOT,
    inference: {
      current: current ?? EMPTY_DASHBOARD_SNAPSHOT.inference.current,
      recent,
    },
  };
}

function makeFakeSource(initial: DashboardSnapshot) {
  let snapshot = initial;
  const listeners = new Set<Parameters<DashboardSnapshotSource['subscribe']>[0]>();
  const source: DashboardSnapshotSource = {
    subscribe(listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    getSnapshot: () => snapshot,
  };
  return {
    source,
    emit(next: DashboardSnapshot) {
      snapshot = next;
      for (const listener of listeners) {
        listener(next);
      }
    },
  };
}

afterEach(() => {
  resetInferenceStore();
});

describe('deriveEventId', () => {
  it('prefers the backend id when present', () => {
    expect(deriveEventId(makeEvent({ id: 'abc' }))).toBe('abc');
  });

  it('falls back to at::endpoint::model when id is absent', () => {
    const event = makeEvent({ at: 'T1' });
    expect(deriveEventId(event)).toBe('T1::/api/generate::gemma4:12b');
  });
});

describe('ingestSnapshotEvents', () => {
  it('skips the zero-value bootstrap event', () => {
    const result = ingestSnapshotEvents([], EMPTY_DASHBOARD_SNAPSHOT);
    expect(result).toHaveLength(0);
  });

  it('upserts by id instead of duplicating on repeated snapshots', () => {
    const inProgress = makeEvent({ id: 'x1', status: PHASE_IN_PROGRESS, tokens: null });
    const completed = makeEvent({ id: 'x1', status: PHASE_COMPLETED });

    const afterFirst = ingestSnapshotEvents([], snapshotWith([inProgress]));
    const afterSecond = ingestSnapshotEvents(afterFirst, snapshotWith([completed]));

    expect(afterSecond).toHaveLength(1);
    expect(afterSecond[0].status).toBe(PHASE_COMPLETED);
  });
});

describe('selectFilteredEvents', () => {
  const events = [
    makeEvent({ id: 'a', model: 'gemma4:12b', endpoint: '/api/generate' }),
    makeEvent({ id: 'b', model: 'llama3', endpoint: '/api/chat', status: PHASE_IN_PROGRESS, tokens: null }),
  ];

  it('filters by case-insensitive query on model and endpoint', () => {
    expect(selectFilteredEvents(events, 'CHAT', 'all')).toHaveLength(1);
    expect(selectFilteredEvents(events, 'gemma', 'all')[0].id).toBe('a');
  });

  it('filters by lifecycle phase', () => {
    expect(selectFilteredEvents(events, '', PHASE_IN_PROGRESS)).toHaveLength(1);
  });
});

describe('selectEventById', () => {
  it('returns null for a null id and for a miss', () => {
    const events = [makeEvent({ id: 'a' })];
    expect(selectEventById(events, null)).toBeNull();
    expect(selectEventById(events, 'missing')).toBeNull();
    expect(selectEventById(events, 'a')?.id).toBe('a');
  });
});

describe('computeAggregates', () => {
  it('returns null metrics when no completed events exist (null != zero)', () => {
    const events = [makeEvent({ id: 'a', status: PHASE_IN_PROGRESS, tokens: null })];
    const agg = computeAggregates(events);
    expect(agg.count).toBe(1);
    expect(agg.avgPerSec).toBeNull();
    expect(agg.p50LatencyMS).toBeNull();
    expect(agg.p95LatencyMS).toBeNull();
  });

  it('computes average tok/s and latency percentiles over completed events', () => {
    const events = [
      makeEvent({ id: 'a', tokens: { promptEvalCount: 0, evalCount: 0, evalDuration: 0, totalDuration: 0, loadDuration: 0, perSec: 40, latencyMS: 1000 } }),
      makeEvent({ id: 'b', tokens: { promptEvalCount: 0, evalCount: 0, evalDuration: 0, totalDuration: 0, loadDuration: 0, perSec: 50, latencyMS: 3000 } }),
    ];
    const agg = computeAggregates(events);
    expect(agg.avgPerSec).toBe(45);
    expect(agg.p50LatencyMS).toBe(1000);
    expect(agg.p95LatencyMS).toBe(3000);
  });
});

describe('connectInferenceStore', () => {
  it('seeds from the current snapshot and ingests subsequent updates', () => {
    const seeded = makeEvent({ id: 'seed' });
    const { source, emit } = makeFakeSource(snapshotWith([seeded]));

    connectInferenceStore(source);
    expect(useInferenceStore.getState().events).toHaveLength(1);

    emit(snapshotWith([seeded, makeEvent({ id: 'next' })]));
    expect(useInferenceStore.getState().events).toHaveLength(2);
  });

  it('is idempotent across repeated connect calls', () => {
    const { source, emit } = makeFakeSource(EMPTY_DASHBOARD_SNAPSHOT);
    connectInferenceStore(source);
    connectInferenceStore(source);

    emit(snapshotWith([makeEvent({ id: 'one' })]));
    expect(useInferenceStore.getState().events).toHaveLength(1);
  });
});

describe('store actions', () => {
  it('updates selection and filters', () => {
    const { select, setQuery, setStatusFilter } = useInferenceStore.getState();
    select('abc');
    setQuery('llama');
    setStatusFilter(PHASE_COMPLETED);

    const state = useInferenceStore.getState();
    expect(state.selectedId).toBe('abc');
    expect(state.query).toBe('llama');
    expect(state.statusFilter).toBe(PHASE_COMPLETED);
  });
});
