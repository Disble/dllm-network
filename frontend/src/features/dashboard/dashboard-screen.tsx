import { DashboardPanel } from './dashboard-panel';
import { useDashboardScreen } from './use-dashboard-screen';
import type { DashboardScreenProps } from './use-dashboard-screen.types';

/**
 * DashboardScreen composes the passive dashboard feature using the runtime snapshot source seam.
 */
export function DashboardScreen({ source, now }: Readonly<DashboardScreenProps>) {
  const viewModel = useDashboardScreen({ source, now });

  return <DashboardPanel viewModel={viewModel} />;
}
