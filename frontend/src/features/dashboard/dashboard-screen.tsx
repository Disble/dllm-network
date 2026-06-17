import { InferenceDetailContainer } from '../inference-detail/inference-detail-container';
import { InferenceFeedContainer } from '../inference-feed/inference-feed-container';
import { RunningModelsContainer } from '../running-models/running-models-container';
import { DashboardPanel } from './dashboard-panel';
import { useDashboardScreen } from './use-dashboard-screen';
import type { DashboardScreenProps } from './use-dashboard-screen.types';

/**
 * DashboardScreen composes the full dashboard UI:
 * - Passive telemetry panel (confirmed + inferred state)
 * - Live inference feed (append-only timeline of captured requests)
 * - Per-request inference detail (selected/most-recent inference metrics)
 * - Enriched running-models panel (per-model size, VRAM, context, TTL)
 */
export function DashboardScreen({ source, now }: Readonly<DashboardScreenProps>) {
  const viewModel = useDashboardScreen({ source, now });

  return (
    <main className="dashboard-root">
      <DashboardPanel viewModel={viewModel} />
      <InferenceFeedContainer source={source} />
      <InferenceDetailContainer source={source} />
      <RunningModelsContainer source={source} />
    </main>
  );
}
