import { InferenceDetailContainer } from '../inference-detail/inference-detail-container';
import { InferenceExplorerContainer } from '../inference-explorer/inference-explorer-container';
import { InferenceKpiContainer } from '../inference-explorer/inference-kpi-container';
import { RunningModelsContainer } from '../running-models/running-models-container';
import { DashboardPanel } from './dashboard-panel';
import { useDashboardScreen } from './use-dashboard-screen';
import type { DashboardScreenProps } from './use-dashboard-screen.types';

/**
 * DashboardScreen composes the dashboard around a DevTools-Network workbench:
 * - Inference workbench: master (virtualized request explorer) + detail
 *   (tabbed per-request inspector) as the primary, hero surface.
 * - Dense secondary telemetry context below the request table.
 */
export function DashboardScreen({ source, now }: Readonly<DashboardScreenProps>) {
  const viewModel = useDashboardScreen({ source, now });

  return (
    <main className="dashboard-root">
      <InferenceKpiContainer source={source} />
      <section className="inference-workbench" aria-label="Inference network">
        <InferenceExplorerContainer source={source} />
        <InferenceDetailContainer source={source} />
      </section>
      <section className="secondary-telemetry-workspace" aria-label="Secondary telemetry">
        <DashboardPanel viewModel={viewModel} />
        <RunningModelsContainer source={source} />
      </section>
      <footer className="dashboard-footer">
        <span>All times shown in UTC</span>
        <span className="dashboard-footer__dot" aria-hidden="true">•</span>
        <span>Auto-refresh off</span>
      </footer>
    </main>
  );
}
