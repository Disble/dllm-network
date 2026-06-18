import { create } from 'zustand';

import { dashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { ingestSnapshotEvents } from './inference-store.helpers';
import type { InferenceStoreState } from './inference-store.types';

/**
 * useInferenceStore is the Zustand read-model for the DevTools-Network inference
 * explorer. It accumulates events (upsert by id), holds selection + filter state,
 * and is fed by a single bridge subscription (see connectInferenceStore). The
 * existing dashboardSnapshotSource stays the runtime/transport ACL; this store
 * only owns React-facing state and never talks to the Wails runtime directly.
 */
export const useInferenceStore = create<InferenceStoreState>((set) => ({
  events: [],
  selectedId: null,
  query: '',
  statusFilter: 'all',
  ingest: (snapshot) =>
    set((state) => ({ events: ingestSnapshotEvents(state.events, snapshot) })),
  select: (id) => set({ selectedId: id }),
  setQuery: (query) => set({ query }),
  setStatusFilter: (statusFilter) => set({ statusFilter }),
}));

/**
 * bridgeUnsubscribe guards the single-subscription bridge so repeated
 * connectInferenceStore calls (e.g. multiple consumers) do not double-subscribe.
 */
let bridgeUnsubscribe: (() => void) | null = null;

/**
 * connectInferenceStore wires a snapshot source into the store as a single
 * bridge: it seeds the store with the current snapshot and then ingests every
 * subsequent update. Idempotent — later calls return the live teardown. Pass a
 * fake source in tests; defaults to the shared runtime source in production.
 */
export function connectInferenceStore(
  source: DashboardSnapshotSource = dashboardSnapshotSource,
): () => void {
  if (bridgeUnsubscribe !== null) {
    return bridgeUnsubscribe;
  }

  const { ingest } = useInferenceStore.getState();
  ingest(source.getSnapshot());
  const unsubscribe = source.subscribe(ingest);

  bridgeUnsubscribe = () => {
    unsubscribe();
    bridgeUnsubscribe = null;
  };

  return bridgeUnsubscribe;
}

/**
 * resetInferenceStore tears down the bridge and clears state. Test-only seam so
 * each test starts from a clean, disconnected store.
 */
export function resetInferenceStore(): void {
  if (bridgeUnsubscribe !== null) {
    bridgeUnsubscribe();
  }
  useInferenceStore.setState({ events: [], selectedId: null, query: '', statusFilter: 'all' });
}
