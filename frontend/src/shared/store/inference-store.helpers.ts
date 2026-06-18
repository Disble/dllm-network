import type { DashboardSnapshot, InferenceEvent } from '../contracts/dashboard-snapshot.types';
import type {
  FilteredInferenceView,
  InferenceAggregates,
  InferenceStatusFilter,
} from './inference-store.types';

/**
 * deriveEventId returns the stable identity used for selection + dedup (R2).
 * Prefers the backend-emitted `id`; falls back to a composite key from
 * at + endpoint + model when it is absent.
 */
export function deriveEventId(event: InferenceEvent): string {
  if (event.id !== undefined && event.id !== '') {
    return event.id;
  }
  return `${event.at}::${event.endpoint}::${event.model}`;
}

/**
 * isRealEvent rejects the zero-value bootstrap event (empty `at`) so the store
 * never accumulates the EMPTY_DASHBOARD_SNAPSHOT placeholder.
 */
function isRealEvent(event: InferenceEvent): boolean {
  return event.at !== '' && event.endpoint !== '';
}

/**
 * upsertEvent inserts a new event or replaces an existing one with the same id,
 * preserving position. Replacement keeps in-progress -> completed transitions
 * stable without duplicating rows or disturbing the user's selection.
 */
function upsertEvent(
  events: readonly InferenceEvent[],
  incoming: InferenceEvent,
): readonly InferenceEvent[] {
  const incomingId = deriveEventId(incoming);
  const index = events.findIndex((existing) => deriveEventId(existing) === incomingId);

  if (index === -1) {
    return [...events, incoming];
  }

  const next = events.slice();
  next[index] = incoming;
  return next;
}

/**
 * ingestSnapshotEvents folds a snapshot's recent list and current event into the
 * accumulated set via upsert-by-id, skipping zero-value placeholders.
 */
export function ingestSnapshotEvents(
  events: readonly InferenceEvent[],
  snapshot: DashboardSnapshot,
): readonly InferenceEvent[] {
  const incoming: InferenceEvent[] = [];
  for (const event of snapshot.inference.recent) {
    if (isRealEvent(event)) {
      incoming.push(event);
    }
  }
  if (isRealEvent(snapshot.inference.current)) {
    incoming.push(snapshot.inference.current);
  }

  return incoming.reduce(upsertEvent, events);
}

/**
 * matchesQuery performs a case-insensitive substring match on model + endpoint.
 */
function matchesQuery(event: InferenceEvent, query: string): boolean {
  const trimmed = query.trim().toLowerCase();
  if (trimmed === '') {
    return true;
  }
  return (
    event.model.toLowerCase().includes(trimmed) ||
    event.endpoint.toLowerCase().includes(trimmed)
  );
}

/**
 * matchesStatusFilter returns true when the event passes the phase filter.
 */
function matchesStatusFilter(event: InferenceEvent, filter: InferenceStatusFilter): boolean {
  return filter === 'all' || event.status === filter;
}

/**
 * selectFilteredEvents applies the query + status filters to the accumulated set.
 */
export function selectFilteredEvents(
  events: readonly InferenceEvent[],
  query: string,
  statusFilter: InferenceStatusFilter,
): readonly InferenceEvent[] {
  return events.filter(
    (event) => matchesQuery(event, query) && matchesStatusFilter(event, statusFilter),
  );
}

/**
 * selectFilteredInferenceView applies the active filters once, then derives the
 * aggregate summary from the exact same filtered subset.
 */
export function selectFilteredInferenceView(
  events: readonly InferenceEvent[],
  query: string,
  statusFilter: InferenceStatusFilter,
): FilteredInferenceView {
  const rows = selectFilteredEvents(events, query, statusFilter);

  return {
    rows,
    aggregates: computeAggregates(rows),
  };
}

/**
 * selectCaptureUnavailable reports whether live capture is unavailable: no events
 * have been accumulated and the snapshot is still in passive-only mode (the
 * WinDivert source is not active/elevated). Mirrors the legacy feed banner gate.
 */
export function selectCaptureUnavailable(
  events: readonly InferenceEvent[],
  captureMode: string,
): boolean {
  return events.length === 0 && captureMode === 'passive-only';
}

/**
 * selectEventById returns the event matching id, or null when absent.
 */
export function selectEventById(
  events: readonly InferenceEvent[],
  id: string | null,
): InferenceEvent | null {
  if (id === null) {
    return null;
  }
  return events.find((event) => deriveEventId(event) === id) ?? null;
}

/**
 * percentile computes the nearest-rank percentile over a numeric sample.
 * Returns null for an empty sample (null != zero invariant).
 */
function percentile(values: readonly number[], p: number): number | null {
  if (values.length === 0) {
    return null;
  }
  const sorted = values.slice().sort((a, b) => a - b);
  const rank = Math.ceil((p / 100) * sorted.length);
  const index = Math.min(Math.max(rank - 1, 0), sorted.length - 1);
  return sorted[index];
}

/**
 * computeAggregates derives the explorer header summary metrics (R5) over the
 * (already filtered) event set. Metrics that require completed events return
 * null when none qualify.
 */
export function computeAggregates(events: readonly InferenceEvent[]): InferenceAggregates {
  const completed = events.filter((event) => event.tokens != null);
  const latencies = completed.map((event) => event.tokens!.latencyMS);

  const avgPerSec = completed.length === 0
    ? null
    : completed.reduce((sum, event) => sum + event.tokens!.perSec, 0) / completed.length;

  const totalEvalCount = completed.reduce((sum, event) => sum + event.tokens!.evalCount, 0);
  const lastUpdated = events.reduce((latest, event) => (event.at > latest ? event.at : latest), '');

  return {
    count: events.length,
    avgPerSec,
    p50LatencyMS: percentile(latencies, 50),
    p95LatencyMS: percentile(latencies, 95),
    totalEvalCount,
    lastUpdated,
  };
}
