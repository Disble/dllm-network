/**
 * DashboardViewModel provides preformatted dashboard values for the passive telemetry screen.
 */
export interface DashboardViewModel {
  readonly confirmedBadgeLabel: string;
  readonly inferredBadgeLabel: string;
  readonly primaryModelValue: string;
  readonly ollamaVersionValue: string;
  readonly processValue: string;
  readonly connectionsValue: string;
  readonly hostValue: string;
  readonly inferredSummary: string;
  readonly confidenceLabel: string;
  readonly evidence: readonly string[];
  readonly passiveLimitations: readonly string[];
  readonly stalenessLabel: string;
  readonly publishedAtLabel: string;
  readonly observedAtLabel: string;
  readonly recentModels: readonly string[];
}

/**
 * DashboardPanelProps defines the readonly panel boundary for dashboard presentation.
 */
export interface DashboardPanelProps {
  readonly viewModel: DashboardViewModel;
}
