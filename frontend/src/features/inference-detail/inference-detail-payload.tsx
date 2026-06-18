import { InferenceDetailBody } from './inference-detail-body';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailPayload renders the captured request body (the prompt + options)
 * for the selected event (the Payload tab).
 */
export function InferenceDetailPayload({ event }: Readonly<InferenceDetailBodyTabProps>) {
  return <InferenceDetailBody body={event.requestBody} truncated={event.requestBodyTruncated} />;
}
