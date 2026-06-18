/**
 * DashboardViewModel provides the compact passive-telemetry summary: a freshness
 * status plus three at-a-glance tiles (collection mode, snapshot time, health).
 * The verbose confirmed/inferred/passive-limit detail was intentionally removed.
 */
export interface DashboardViewModel {
  /** Formatted publish timestamp for the "Published …" line. */
  readonly publishedAtLabel: string;
  /** Freshness label shown in the header pill ("Fresh"/"Stale passive snapshot"). */
  readonly stalenessLabel: string;
  /** Whether the snapshot is fresh (drives the freshness pill state). */
  readonly isFresh: boolean;
  /** Humanised collection mode tile value (e.g. "Passive-only"). */
  readonly collectionModeLabel: string;
  /** Snapshot time tile value (e.g. "2026-06-18 14:25:43Z"). */
  readonly snapshotTimeLabel: string;
  /** Collector health tile value (e.g. "Healthy"). */
  readonly healthLabel: string;
}

/**
 * DashboardPanelProps defines the readonly panel boundary for dashboard presentation.
 */
export interface DashboardPanelProps {
  readonly viewModel: DashboardViewModel;
}
