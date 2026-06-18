import { NOT_CAPTURED_LABEL } from './inference-detail.constants';
import { InferenceDetailCodeBlock } from './inference-detail-code-block';
import { formatJsonPretty } from './inference-detail-body.helpers';
import type { InferenceDetailBodyProps } from './inference-detail.types';

/**
 * InferenceDetailBody renders a raw HTTP body, shared by the Payload and
 * Response tabs. Delegates to the shared code viewer (Pretty/Raw toggle + Copy),
 * pre-computing the pretty form when the body is JSON. Renders an honest "not
 * captured" notice (≠ empty) when the body is absent.
 */
export function InferenceDetailBody({ body, truncated }: Readonly<InferenceDetailBodyProps>) {
  if (body === undefined || body === '') {
    return <p className="inference-detail__not-captured">{NOT_CAPTURED_LABEL}</p>;
  }

  return <InferenceDetailCodeBlock raw={body} pretty={formatJsonPretty(body)} truncated={truncated} />;
}
