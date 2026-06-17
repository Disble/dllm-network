/**
 * Phase describes the lifecycle state of a captured inference request.
 * Mirrors internal/telemetry/inference.Phase (iota: 0=InProgress, 1=Completed, 2=MetadataOnly).
 */
export type InferencePhase = 0 | 1 | 2;

/** PhaseInProgress — request is streaming; no derived metrics available. */
export const PHASE_IN_PROGRESS: InferencePhase = 0;
/** PhaseCompleted — terminal done:true line observed; all metrics available. */
export const PHASE_COMPLETED: InferencePhase = 1;
/** PhaseMetadataOnly — non-inference endpoint (e.g. /api/tags); token metrics structurally unavailable. */
export const PHASE_METADATA_ONLY: InferencePhase = 2;

/**
 * TokenStats holds the raw Ollama response performance counters and derived metrics.
 * Mirrors internal/telemetry/inference.TokenStats. A null value means metrics are unavailable — NOT zero.
 */
export interface TokenStats {
  /** prompt_eval_count from the terminal done:true NDJSON line. */
  readonly promptEvalCount: number;
  /** eval_count (generated tokens) from the terminal done:true NDJSON line. */
  readonly evalCount: number;
  /** eval_duration in nanoseconds from the terminal done:true NDJSON line. */
  readonly evalDuration: number;
  /** total_duration in nanoseconds from the terminal done:true NDJSON line. */
  readonly totalDuration: number;
  /** load_duration in nanoseconds from the terminal done:true NDJSON line. */
  readonly loadDuration: number;
  /** Derived: eval_count / (eval_duration in seconds). Zero if eval_duration is zero. */
  readonly perSec: number;
  /** Derived: total_duration in milliseconds. */
  readonly latencyMS: number;
}

/**
 * InferenceEvent is the frontend mirror of internal/telemetry/inference.Inference.
 * JSON field names match the Go json tags exactly.
 */
export interface InferenceEvent {
  /** Wall-clock time the exchange was processed (RFC3339). Mirrors json:"at" via Go time.Time marshalling. */
  readonly at: string;
  /** HTTP path (e.g. "/api/generate"). Mirrors json:"endpoint" — Go uses lowercase by convention. */
  readonly endpoint: string;
  /** HTTP method (e.g. "POST"). */
  readonly method: string;
  /** Model name from request JSON body. */
  readonly model: string;
  /** Byte length of request body (prompt + options). */
  readonly promptSize: number;
  /** True when response was an NDJSON stream. */
  readonly streaming: boolean;
  /** Lifecycle phase. Mirrors inference.Phase (int). */
  readonly status: InferencePhase;
  /**
   * Token performance metrics. Null when unavailable (InProgress or MetadataOnly).
   * Callers MUST check for null before reading any field.
   */
  readonly tokens: TokenStats | null;
}

/**
 * InferenceState holds the live and recent inference activity.
 * Mirrors internal/dashboard.InferenceState.
 */
export interface InferenceState {
  /** Most recent in-progress or just-completed inference. Zero-value when no capture data yet. */
  readonly current: InferenceEvent;
  /** Last N completed inference events in chronological order. Empty when capture is disabled. */
  readonly recent: readonly InferenceEvent[];
}

/**
 * RunningModelView is the enriched per-model view.
 * Mirrors internal/dashboard.RunningModelView JSON tags.
 */
export interface RunningModelView {
  readonly name: string;
  readonly size: number;
  /** Mirrors json:"sizeVram" (Go field SizeVRAM). */
  readonly sizeVram: number;
  readonly parameterSize: string;
  readonly quantizationLevel: string;
  readonly contextLength: number;
  /** ISO-8601 timestamp. Mirrors json:"expiresAt". */
  readonly expiresAt: string;
}

/**
 * DashboardSnapshot mirrors the backend dashboard snapshot payload for the React dashboard.
 */
export interface DashboardSnapshot {
  readonly publishedAt: string;
  readonly confirmed: ConfirmedState;
  readonly inferred: InferredState;
  readonly recent: RecentState;
  readonly health: CollectorHealth;
  readonly passive: PassiveState;
  /** Live and recent inference activity from the capture pipeline. Zero-value when capture is disabled. */
  readonly inference: InferenceState;
}

/**
 * ConfirmedState contains passive telemetry that the backend can confirm directly.
 */
export interface ConfirmedState {
  readonly ollama: ConfirmedOllamaState;
  readonly system: ConfirmedSystemState;
}

/**
 * ConfirmedOllamaState describes confirmed Ollama runtime state.
 */
export interface ConfirmedOllamaState {
  readonly status: string;
  readonly reachable: boolean;
  readonly version: string;
  readonly primaryModel: string;
  readonly runningModels: readonly string[];
  /**
   * Enriched per-model details. Mirrors ConfirmedOllamaState.RunningModelDetails from the backend.
   * Additive alongside runningModels for back-compat; present when backend sends enriched data.
   */
  readonly runningModelDetails?: readonly RunningModelView[];
  readonly catalogModelCount: number;
  readonly observedAt: string;
  readonly lastConfirmedAt: string;
}

/**
 * ConfirmedSystemState describes confirmed host-side process and host metrics.
 */
export interface ConfirmedSystemState {
  readonly observedAt: string;
  readonly process: ConfirmedProcessState;
  readonly connections: ConfirmedConnectionsState;
  readonly host: ConfirmedHostState;
}

/**
 * ConfirmedProcessState describes confirmed Ollama process information.
 */
export interface ConfirmedProcessState {
  readonly status: string;
  readonly found: boolean;
  readonly pid: number;
  readonly cpuPercent: number;
  readonly rssBytes: number;
}

/**
 * ConfirmedConnectionsState describes confirmed owned loopback connection state.
 */
export interface ConfirmedConnectionsState {
  readonly status: string;
  readonly count: number;
}

/**
 * ConfirmedHostState describes confirmed host metrics.
 */
export interface ConfirmedHostState {
  readonly status: string;
  readonly cpuPercent: number;
  readonly memoryUsedBytes: number;
  readonly memoryTotalBytes: number;
}

/**
 * InferredState contains the current inferred activity plus recent inferred history.
 */
export interface InferredState {
  readonly current: InferredActivity;
  readonly recent: readonly InferredActivity[];
}

/**
 * InferredActivity carries passive-only activity claims with explicit evidence.
 */
export interface InferredActivity {
  readonly kind: string;
  readonly truth: string;
  readonly model: string;
  readonly confidence: string;
  readonly observedAt: string;
  readonly evidence: readonly EvidenceItem[];
}

/**
 * EvidenceItem explains why an inferred activity claim exists.
 */
export interface EvidenceItem {
  readonly kind: string;
  readonly detail: string;
}

/**
 * RecentState keeps lightweight confirmed history for the dashboard.
 */
export interface RecentState {
  readonly confirmedModels: readonly RecentConfirmedModel[];
}

/**
 * RecentConfirmedModel stores one confirmed model observation.
 */
export interface RecentConfirmedModel {
  readonly observedAt: string;
  readonly model: string;
}

/**
 * CollectorHealth reports backend source health at the time of publication.
 */
export interface CollectorHealth {
  readonly ollama: HealthState;
  readonly process: HealthState;
  readonly connections: HealthState;
  readonly host: HealthState;
}

/**
 * HealthState reports the health of one telemetry source.
 */
export interface HealthState {
  readonly status: string;
  readonly healthy: boolean;
  readonly supported: boolean;
  readonly observedAt: string;
  readonly error: string;
}

/**
 * PassiveState describes hard product limits of passive-only telemetry.
 */
export interface PassiveState {
  readonly mode: string;
  readonly exactRequestLatencyAvailable: boolean;
  readonly exactTokenCountsAvailable: boolean;
  readonly exactPayloadAvailable: boolean;
  readonly exactStatusAvailable: boolean;
  readonly exactStreamingChunksAvailable: boolean;
  readonly notes: readonly string[];
}
