import type { InferenceDetailOverviewProps } from './inference-detail.types';

/**
 * InferenceDetailOverview renders the precomputed metric fields for the selected
 * request as a definition list (the Overview tab).
 */
export function InferenceDetailOverview({ viewModel: vm }: Readonly<InferenceDetailOverviewProps>) {
  return (
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
  );
}
