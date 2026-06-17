import type { InferenceDetailPanelProps } from './inference-detail.types';

/**
 * InferenceDetailPanel is a pure presentational component that renders the per-request
 * inference detail view from a precomputed view model. Never imports infrastructure.
 */
export function InferenceDetailPanel({ viewModel: vm }: Readonly<InferenceDetailPanelProps>) {
  return (
    <section className="inference-detail" aria-label="Inference detail">
      <header className="inference-detail__header">
        <span className="inference-detail__model">{vm.model}</span>
        <span className="inference-detail__status">{vm.statusLabel}</span>
      </header>
      <dl className="inference-detail__fields">
        <dt>Endpoint</dt>
        <dd>{vm.endpoint}</dd>

        <dt>Method</dt>
        <dd>{vm.method}</dd>

        <dt>Prompt size</dt>
        <dd>{vm.promptSizeLabel}</dd>

        <dt>Tokens/sec</dt>
        <dd>{vm.tokenRateLabel}</dd>

        <dt>Latency</dt>
        <dd>{vm.latencyLabel}</dd>

        <dt>Prompt eval count</dt>
        <dd>{vm.promptEvalCountLabel}</dd>

        <dt>Eval count</dt>
        <dd>{vm.evalCountLabel}</dd>

        <dt>Timestamp</dt>
        <dd>{vm.timestampLabel}</dd>
      </dl>
    </section>
  );
}
