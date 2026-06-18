import { InferenceDetailBody } from './inference-detail-body';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailResponse renders the captured response body (assembled NDJSON
 * or final payload) for the selected event (the Response tab).
 */
export function InferenceDetailResponse({ event }: Readonly<InferenceDetailBodyTabProps>) {
  return <InferenceDetailBody body={event.responseBody} truncated={event.responseBodyTruncated} />;
}
