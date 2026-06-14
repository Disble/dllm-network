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
