import type { DashboardPanelProps } from './dashboard-view-model.types';

/**
 * DashboardPanel renders the compact passive-telemetry summary: a freshness
 * header plus three at-a-glance tiles (collection mode, snapshot time, health).
 * Verbose confirmed/inferred detail and passive-limit warnings were removed.
 */
export function DashboardPanel({ viewModel: vm }: Readonly<DashboardPanelProps>) {
  return (
    <section className="telemetry-panel" aria-label="Passive telemetry context">
      <header className="telemetry-panel__header">
        <div className="telemetry-panel__heading">
          <p className="eyebrow">Passive-only telemetry</p>
          <h2 className="telemetry-panel__title">dllm-network</h2>
          <p className="telemetry-panel__published">Published {vm.publishedAtLabel}</p>
        </div>
        <span className={`telemetry-panel__freshness${vm.isFresh ? ' telemetry-panel__freshness--fresh' : ''}`}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
            <path d="M3 12a9 9 0 0 1 9-9 9 9 0 0 1 6.36 2.64L21 8" />
            <path d="M21 3v5h-5" />
          </svg>
          {vm.stalenessLabel}
        </span>
      </header>

      <dl className="telemetry-tiles">
        <div className="telemetry-tile">
          <svg className="telemetry-tile__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
            <ellipse cx="12" cy="5" rx="9" ry="3" />
            <path d="M3 5v14a9 3 0 0 0 18 0V5" />
            <path d="M3 12a9 3 0 0 0 18 0" />
          </svg>
          <div className="telemetry-tile__body">
            <dt>Collection mode</dt>
            <dd>{vm.collectionModeLabel}</dd>
          </div>
        </div>

        <div className="telemetry-tile">
          <svg className="telemetry-tile__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
            <circle cx="12" cy="12" r="10" />
            <polyline points="12 6 12 12 16 14" />
          </svg>
          <div className="telemetry-tile__body">
            <dt>Snapshot time</dt>
            <dd>{vm.snapshotTimeLabel}</dd>
          </div>
        </div>

        <div className="telemetry-tile">
          <svg className="telemetry-tile__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
            <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
          </svg>
          <div className="telemetry-tile__body">
            <dt>Status</dt>
            <dd>{vm.healthLabel}</dd>
          </div>
        </div>
      </dl>
    </section>
  );
}
