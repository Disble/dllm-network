import type { DashboardSnapshot, InferenceEvent, InferencePhase } from '../contracts/dashboard-snapshot.types';

/**
 * InferenceStatusFilter narrows the explorer table by lifecycle phase.
 * 'all' disables status filtering.
 */
export type InferenceStatusFilter = 'all' | InferencePhase;

/**
 * InferenceAggregates holds derived summary metrics for the explorer header (R5).
 * Numeric fields are null — NOT zero — when no completed event supplies them
 * (null != zero invariant).
 */
export interface InferenceAggregates {
  /** Total accumulated events in the session. */
  readonly count: number;
  /** Mean tokens/sec across completed events, or null when none completed. */
  readonly avgPerSec: number | null;
  /** Median (p50) end-to-end latency in ms, or null when none completed. */
  readonly p50LatencyMS: number | null;
  /** 95th percentile latency in ms, or null when none completed. */
  readonly p95LatencyMS: number | null;
  /** Sum of generated token counts across completed events. */
  readonly totalEvalCount: number;
  /** Timestamp (RFC3339) of the most recent event, or '' when none. */
  readonly lastUpdated: string;
}

/**
 * InferenceStoreState is the full Zustand store shape: accumulated read-model
 * state plus the actions that mutate it. The store is fed by a single bridge
 * subscription to the snapshot source (see connectInferenceStore).
 */
export interface InferenceStoreState {
  /** Accumulated inference events, deduplicated/upserted by stable id, chronological. */
  readonly events: readonly InferenceEvent[];
  /** Currently selected event id for master-detail (R2), or null when none. */
  readonly selectedId: string | null;
  /** Free-text filter applied to model + endpoint (R4). */
  readonly query: string;
  /** Lifecycle-phase filter (R4). */
  readonly statusFilter: InferenceStatusFilter;
  /** Latest passive-limit mode from the snapshot ('passive-only' | 'capture-active'). */
  readonly captureMode: string;
  /** Latest passive-limit notes from the snapshot (used for the capture-unavailable banner). */
  readonly passiveNotes: readonly string[];
  /** Merge a snapshot's inference events into the accumulated list (upsert by id). */
  // eslint-disable-next-line no-unused-vars -- function-type param documents the action contract.
  readonly ingest: (snapshot: DashboardSnapshot) => void;
  /** Set (or clear with null) the selected event id. */
  // eslint-disable-next-line no-unused-vars -- function-type param documents the action contract.
  readonly select: (id: string | null) => void;
  /** Set the free-text query filter. */
  // eslint-disable-next-line no-unused-vars -- function-type param documents the action contract.
  readonly setQuery: (query: string) => void;
  /** Set the lifecycle-phase filter. */
  // eslint-disable-next-line no-unused-vars -- function-type param documents the action contract.
  readonly setStatusFilter: (filter: InferenceStatusFilter) => void;
}
