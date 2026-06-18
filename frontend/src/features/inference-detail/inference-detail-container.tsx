import { InferenceDetailPanel } from './inference-detail-panel';
import { useInferenceDetail } from './use-inference-detail';
import type { InferenceDetailContainerProps } from './inference-detail.types';

/**
 * InferenceDetailContainer is the detail side of the DevTools-Network layout:
 * it reads the selected event from the shared store and renders the tabbed
 * detail panel. Follows the container/presentational split.
 */
export function InferenceDetailContainer({ source }: Readonly<InferenceDetailContainerProps>) {
  const { event, overview } = useInferenceDetail(source);

  return <InferenceDetailPanel event={event} overview={overview} />;
}
