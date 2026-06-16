import { EMPTY_DASHBOARD_SNAPSHOT } from '../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot } from '../shared/contracts/dashboard-snapshot.types';

const DASHBOARD_SNAPSHOT_EVENT = 'dashboard:snapshot';

let sharedSource: DashboardSnapshotSource | null = null;

// eslint-disable-next-line no-unused-vars -- Function type documents the Wails runtime event contract used by infrastructure only.
type RuntimeEventsOn = (eventName: string, callback: (...payload: readonly unknown[]) => void) => () => void;

/**
 * DashboardSnapshotSource subscribes to dashboard snapshot updates from a runtime-specific transport.
 */
export interface DashboardSnapshotSource {
  // eslint-disable-next-line no-unused-vars -- Type-only callback parameter documents the snapshot contract.
  readonly subscribe: (listener: (snapshot: DashboardSnapshot) => void) => () => void;
  readonly getSnapshot: () => DashboardSnapshot;
}

/**
 * createDashboardSnapshotSource returns the singleton runtime-backed source for dashboard snapshots.
 */
export function createDashboardSnapshotSource(): DashboardSnapshotSource {
  if (sharedSource !== null) {
    return sharedSource;
  }

  let currentSnapshot = EMPTY_DASHBOARD_SNAPSHOT;
  // eslint-disable-next-line no-unused-vars -- Listener type documents the snapshot contract for subscribers.
  const listeners = new Set<(snapshot: DashboardSnapshot) => void>();
  let runtimeUnsubscribe: (() => void) | null = null;

  const handleRuntimeSnapshot = (...payload: readonly unknown[]) => {
    const nextSnapshot = payload[0];

    if (nextSnapshot === undefined) {
      return;
    }

    currentSnapshot = nextSnapshot as DashboardSnapshot;

    for (const listener of listeners) {
      listener(currentSnapshot);
    }
  };

  const releaseRuntimeListener = () => {
    if (runtimeUnsubscribe === null) {
      return;
    }

    const unsubscribe = runtimeUnsubscribe;
    runtimeUnsubscribe = null;
    unsubscribe();
  };

  const ensureRuntimeListener = () => {
    if (runtimeUnsubscribe !== null) {
      return;
    }

    const runtimeBridge = (window as typeof window & { runtime?: { EventsOn?: RuntimeEventsOn } }).runtime;
    const runtimeEventsOn = runtimeBridge?.EventsOn;

    if (runtimeEventsOn === undefined) {
      // The Wails runtime is only injected inside the desktop webview. When the
      // frontend runs in a plain browser (e.g. vite dev), degrade to a no-op so
      // the dashboard still mounts with the passive bootstrap snapshot.
      return;
    }

    runtimeUnsubscribe = runtimeEventsOn(DASHBOARD_SNAPSHOT_EVENT, handleRuntimeSnapshot);
  };

  sharedSource = {
    subscribe(listener) {
      listeners.add(listener);
      ensureRuntimeListener();

      let subscribed = true;

      return () => {
        if (!subscribed) {
          return;
        }

        subscribed = false;
        listeners.delete(listener);

        if (listeners.size === 0) {
          releaseRuntimeListener();
        }
      };
    },
    getSnapshot() {
      return currentSnapshot;
    },
  };

  return sharedSource;
}

/**
 * dashboardSnapshotSource exposes the shared runtime-backed snapshot source to feature hooks.
 */
export const dashboardSnapshotSource = createDashboardSnapshotSource();
