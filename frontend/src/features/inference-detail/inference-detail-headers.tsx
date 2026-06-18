import { NOT_CAPTURED_LABEL } from './inference-detail.constants';
import { InferenceDetailHeaderGroup } from './inference-detail-header-group';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailHeaders renders the captured request/response headers in order
 * (the Headers tab), or an honest not-captured notice when none are present.
 */
export function InferenceDetailHeaders({ event }: Readonly<InferenceDetailBodyTabProps>) {
  const requestHeaders = event.requestHeaders ?? [];
  const responseHeaders = event.responseHeaders ?? [];

  if (requestHeaders.length === 0 && responseHeaders.length === 0) {
    return <p className="inference-detail__not-captured">{NOT_CAPTURED_LABEL}</p>;
  }

  return (
    <div className="inference-detail__headers">
      <InferenceDetailHeaderGroup title="Request headers" headers={requestHeaders} />
      <InferenceDetailHeaderGroup title="Response headers" headers={responseHeaders} />
    </div>
  );
}
