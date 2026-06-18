import { DashboardScreen } from '../features/dashboard/dashboard-screen';
import { AppShell } from '../shared/ui/layout/app-shell';

/**
 * DashboardApp is the app-layer composition root for the passive telemetry dashboard.
 * It frames the dashboard screen in the application shell (title bar + nav rail).
 */
export function DashboardApp() {
  return (
    <AppShell>
      <DashboardScreen />
    </AppShell>
  );
}

