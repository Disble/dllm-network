import type { RunningModelCardProps } from './running-models.types';

/**
 * RunningModelCard renders a loaded model as a compact row (icon + name) with a
 * trailing affordance. Enriched detail (params, quant, size, VRAM, context) is
 * available in the view model for a future expand but is kept off the summary.
 */
export function RunningModelCard({ viewModel: vm }: Readonly<RunningModelCardProps>) {
  return (
    <article className="model-row" aria-label={`Running model: ${vm.name}`}>
      <svg className="model-row__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
        <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" />
        <path d="m3.3 7 8.7 5 8.7-5" />
        <path d="M12 22V12" />
      </svg>
      <span className="model-row__name">{vm.name}</span>
      <svg className="model-row__chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
        <polyline points="9 18 15 12 9 6" />
      </svg>
    </article>
  );
}
