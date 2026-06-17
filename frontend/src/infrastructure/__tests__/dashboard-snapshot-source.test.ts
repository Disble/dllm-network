import { describe, expect, it, vi } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../../shared/contracts/dashboard-snapshot.types';

// eslint-disable-next-line no-unused-vars -- Function type documents Wails event payload delivery.
type RuntimeListener = (...payload: readonly unknown[]) => void;

describe('dashboardSnapshotSource', () => {
  it('returns the passive bootstrap snapshot before the first runtime event arrives', async () => {
    const module = await loadDashboardSnapshotSourceModule();
    const source = module.createDashboardSnapshotSource();

    expect(source.getSnapshot()).toEqual(EMPTY_DASHBOARD_SNAPSHOT);
  });

  it('degrades to a no-op subscription when the Wails runtime is unavailable', async () => {
    const module = await loadDashboardSnapshotSourceModuleWithoutRuntime();
    const source = module.createDashboardSnapshotSource();
    const listener = vi.fn();

    let unsubscribe: () => void = () => {};
    expect(() => {
      unsubscribe = source.subscribe(listener);
    }).not.toThrow();

    expect(source.getSnapshot()).toEqual(EMPTY_DASHBOARD_SNAPSHOT);
    expect(listener).not.toHaveBeenCalled();
    expect(() => unsubscribe()).not.toThrow();
  });

  it('delivers dashboard:snapshot payloads through one shared runtime subscription', async () => {
    const runtime = createRuntimeEventsOnMock();
    const module = await loadDashboardSnapshotSourceModule(runtime);
    const source = module.createDashboardSnapshotSource();
    const firstListener = vi.fn();
    const secondListener = vi.fn();

    const unsubscribeFirst = source.subscribe(firstListener);
    const unsubscribeSecond = source.subscribe(secondListener);

    expect(runtime.eventsOn).toHaveBeenCalledTimes(1);
    expect(runtime.eventsOn).toHaveBeenCalledWith('dashboard:snapshot', expect.any(Function));

    const snapshot = createSnapshot('mistral');
    runtime.emit(snapshot);

    expect(source.getSnapshot()).toEqual(snapshot);
    expect(firstListener).toHaveBeenCalledWith(snapshot);
    expect(secondListener).toHaveBeenCalledWith(snapshot);

    unsubscribeFirst();
    unsubscribeSecond();
  });

  it('keeps the runtime listener alive until the last subscriber leaves', async () => {
    const runtime = createRuntimeEventsOnMock();
    const module = await loadDashboardSnapshotSourceModule(runtime);
    const source = module.createDashboardSnapshotSource();

    const unsubscribeFirst = source.subscribe(vi.fn());
    const unsubscribeSecond = source.subscribe(vi.fn());

    unsubscribeFirst();

    expect(runtime.unsubscribe).not.toHaveBeenCalled();

    unsubscribeSecond();

    expect(runtime.unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('cleans up the last runtime listener once and acquires a fresh listener after resubscribe', async () => {
    const runtime = createRuntimeEventsOnMock();
    const module = await loadDashboardSnapshotSourceModule(runtime);
    const source = module.createDashboardSnapshotSource();

    const unsubscribe = source.subscribe(vi.fn());
    unsubscribe();
    unsubscribe();

    expect(runtime.unsubscribe).toHaveBeenCalledTimes(1);

    source.subscribe(vi.fn())();

    expect(runtime.eventsOn).toHaveBeenCalledTimes(2);
    expect(runtime.unsubscribe).toHaveBeenCalledTimes(2);
  });
});

const loadDashboardSnapshotSourceModule = async (runtime = createRuntimeEventsOnMock()) => {
  vi.resetModules();
  Object.assign(window, {
    runtime: {
      EventsOn: runtime.eventsOn,
    },
  });

  return import('../dashboard-snapshot-source');
};

const loadDashboardSnapshotSourceModuleWithoutRuntime = async () => {
  vi.resetModules();
  delete (window as typeof window & { runtime?: unknown }).runtime;

  return import('../dashboard-snapshot-source');
};

const createRuntimeEventsOnMock = () => {
  let listener: RuntimeListener | null = null;
  const unsubscribe = vi.fn();
  const eventsOn = vi.fn((_eventName: string, nextListener: RuntimeListener) => {
    listener = nextListener;

    return () => {
      listener = null;
      unsubscribe();
    };
  });

  return {
    emit(snapshot: DashboardSnapshot) {
      listener?.(snapshot);
    },
    eventsOn,
    unsubscribe,
  };
};

const createSnapshot = (model: string): DashboardSnapshot => ({
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
      kind: 'inferred-model-changed',
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
