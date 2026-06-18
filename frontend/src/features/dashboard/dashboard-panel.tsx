import type { DashboardPanelProps } from './dashboard-view-model.types';

/**
 * DashboardPanel renders the passive telemetry dashboard from a precomputed view model.
 */
export function DashboardPanel({ viewModel }: Readonly<DashboardPanelProps>) {
  return (
    <section className="dashboard-shell" aria-label="Passive telemetry context">
      <header className="dashboard-header">
        <div>
          <p className="eyebrow">Passive-only telemetry</p>
          <h1>Ollama Telemetry</h1>
          <p className="body-copy">Published {viewModel.publishedAtLabel}</p>
        </div>
        <p className="status-pill">{viewModel.stalenessLabel}</p>
      </header>

      <section className="dashboard-grid">
        <article className="metric-card metric-card--confirmed">
          <p className="section-label">{viewModel.confirmedBadgeLabel}</p>
          <h2>{viewModel.primaryModelValue}</h2>
          <ul className="metric-list">
            <li>Ollama version: {viewModel.ollamaVersionValue}</li>
            <li>Process: {viewModel.processValue}</li>
            <li>Connections: {viewModel.connectionsValue}</li>
            <li>Host: {viewModel.hostValue}</li>
            <li>Observed: {viewModel.observedAtLabel}</li>
          </ul>
        </article>

        <article className="metric-card accent-card metric-card--inferred">
          <p className="section-label">{viewModel.inferredBadgeLabel}</p>
          <h2>{viewModel.inferredSummary}</h2>
          <p className="confidence-text">{viewModel.confidenceLabel}</p>
          <ul className="metric-list">
            {viewModel.evidence.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </article>

        <article className="metric-card wide-card metric-card--limits">
          <p className="section-label">Passive limits</p>
          <ul className="metric-list">
            {viewModel.passiveLimitations.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </article>

        <article className="metric-card wide-card metric-card--history">
          <p className="section-label">Recent confirmed models</p>
          <ul className="metric-list">
            {viewModel.recentModels.length > 0 ? (
              viewModel.recentModels.map((item) => <li key={item}>{item}</li>)
            ) : (
              <li>No confirmed model history yet.</li>
            )}
          </ul>
        </article>
      </section>
    </section>
  );
}
