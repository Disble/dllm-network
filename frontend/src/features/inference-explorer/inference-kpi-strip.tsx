import type { InferenceKpiStripProps } from './inference-explorer.types';

/**
 * InferenceKpiStrip renders the full-width top metrics strip (requests, tok/s,
 * latency percentiles, eval count, last-updated) above the request workbench.
 */
export function InferenceKpiStrip({ items }: Readonly<InferenceKpiStripProps>) {
  return (
    <section className="kpi-strip" aria-label="Inference metrics">
      {items.map((item) => (
        <div key={item.label} className="kpi-strip__item">
          <p className="kpi-strip__label">{item.label}</p>
          <p className="kpi-strip__value">{item.value}</p>
          <p className="kpi-strip__caption">{item.caption}</p>
        </div>
      ))}
    </section>
  );
}
