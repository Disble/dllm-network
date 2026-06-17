import type { RunningModelCardProps } from './running-models.types';

/**
 * RunningModelCard renders enriched per-model information as a presentational card.
 * Receives a precomputed view model; never imports infrastructure or raw types.
 */
export function RunningModelCard({ viewModel: vm }: Readonly<RunningModelCardProps>) {
  return (
    <article className="running-model-card" aria-label={`Running model: ${vm.name}`}>
      <header className="running-model-card__name">{vm.name}</header>
      <dl className="running-model-card__fields">
        <dt>Parameters</dt>
        <dd>{vm.parameterSize}</dd>

        <dt>Quantization</dt>
        <dd>{vm.quantizationLevel}</dd>

        <dt>Size</dt>
        <dd>{vm.sizeLabel}</dd>

        <dt>VRAM</dt>
        <dd>{vm.sizeVramLabel}</dd>

        <dt>Context</dt>
        <dd>{vm.contextLengthLabel}</dd>

        <dt>Expires</dt>
        <dd>{vm.expiresAtLabel}</dd>
      </dl>
    </article>
  );
}
