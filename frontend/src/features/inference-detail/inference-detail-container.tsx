import { InferenceDetailPanel } from './inference-detail-panel';
import { useInferenceDetail } from './use-inference-detail';
import type { InferenceDetailContainerProps } from './inference-detail.types';

/**
 * InferenceDetailContainer is a container component that subscribes to the snapshot source
 * and renders the per-request inference detail via InferenceDetailPanel.
 * Follows the container/presentational split: this component manages data; the panel renders it.
 */
export function InferenceDetailContainer({ source }: Readonly<InferenceDetailContainerProps>) {
  const viewModel = useInferenceDetail(source);

  return <InferenceDetailPanel viewModel={viewModel} />;
}
