import { useSyncExternalStore, useRef } from 'react';
import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';

/**
 * useInferenceFeed subscribes to snapshot updates and returns the accumulated
 * inference event list. Events are appended as new snapshots arrive — older
 * events are never discarded within the same session.
 */
export function useInferenceFeed(source?: DashboardSnapshotSource): readonly InferenceEvent[] {
  const resolvedSource = source ?? dashboardSnapshotSource;

  // Mutable refs hold accumulated state across renders without causing re-renders themselves.
  const feedRef = useRef<readonly InferenceEvent[]>([]);
  // Use lazy init for the Set to avoid re-creation on every render.
  const seenKeysRef = useRef<Set<string>>(null);
  if (seenKeysRef.current === null) {
    seenKeysRef.current = new Set<string>();
  }
  const feedSnapshotRef = useRef<readonly InferenceEvent[]>([]);

  const getSnapshot = (): readonly InferenceEvent[] => {
    const incoming = resolvedSource.getSnapshot().inference?.recent ?? [];

    let changed = false;
    for (const event of incoming) {
      const key = `${event.at}::${event.endpoint}::${event.model}`;
      if (!seenKeysRef.current!.has(key)) {
        seenKeysRef.current!.add(key);
        feedRef.current = [...feedRef.current, event];
        changed = true;
      }
    }

    if (changed) {
      feedSnapshotRef.current = feedRef.current;
    }

    return feedSnapshotRef.current;
  };

  return useSyncExternalStore(resolvedSource.subscribe, getSnapshot);
}
