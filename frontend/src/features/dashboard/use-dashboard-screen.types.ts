import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';

/**
 * UseDashboardScreenOptions defines injectable seams for dashboard runtime source and clock control.
 */
export interface UseDashboardScreenOptions {
  readonly source?: DashboardSnapshotSource;
  readonly now?: Date;
}

/**
 * DashboardScreenProps exposes optional test seams without leaking runtime imports into the screen component.
 */
export interface DashboardScreenProps extends UseDashboardScreenOptions {}
