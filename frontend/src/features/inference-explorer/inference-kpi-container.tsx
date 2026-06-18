import { InferenceKpiStrip } from './inference-kpi-strip';
import { useInferenceMetrics } from './use-inference-metrics';
import type { InferenceExplorerContainerProps } from './inference-explorer.types';

/**
 * InferenceKpiContainer wires the shared store metrics into the top KPI strip.
 * Reuses the explorer's injectable source seam.
 */
export function InferenceKpiContainer({ source }: Readonly<InferenceExplorerContainerProps>) {
  const { items } = useInferenceMetrics(source);

  return <InferenceKpiStrip items={items} />;
}
