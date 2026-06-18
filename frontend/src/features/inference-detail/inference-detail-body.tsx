import { NOT_CAPTURED_LABEL } from './inference-detail.constants';
import type { InferenceDetailBodyProps } from './inference-detail.types';

/**
 * InferenceDetailBody renders a raw HTTP body as a code block, shared by the
 * Payload and Response tabs. Renders an honest "not captured" notice (≠ empty)
 * when the body is absent — passive capture has not surfaced it yet.
 */
export function InferenceDetailBody({ body, truncated }: Readonly<InferenceDetailBodyProps>) {
  if (body === undefined || body === '') {
    return <p className="inference-detail__not-captured">{NOT_CAPTURED_LABEL}</p>;
  }

  return (
    <div className="inference-detail__body">
      <pre className="inference-detail__code">{body}</pre>
      {truncated ? <p className="inference-detail__truncated">Truncated at capture limit.</p> : null}
    </div>
  );
}
