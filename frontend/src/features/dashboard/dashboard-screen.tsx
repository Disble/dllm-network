import { DashboardPanel } from './dashboard-panel';
import { useDashboardScreen } from './use-dashboard-screen';

/**
 * DashboardScreen composes the passive dashboard feature using the runtime snapshot source seam.
 */
export function DashboardScreen() {
  const viewModel = useDashboardScreen();

  return <DashboardPanel viewModel={viewModel} />;
}
