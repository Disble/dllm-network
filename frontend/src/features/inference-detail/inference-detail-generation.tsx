import { useState } from 'react';

import { NOT_CAPTURED_LABEL, UNAVAILABLE_LABEL } from './inference-detail.constants';
import { InferenceDetailCodeBlock } from './inference-detail-code-block';
import { InferenceDetailContextToggle } from './inference-detail-context-toggle';
import { buildGenerationView } from './inference-detail-generation-view-model.helpers';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailGeneration renders the LLM-aware view of an Ollama
 * generate/chat response (the Generation tab): the pretty-printed model output,
 * a compact `context` token summary (count + collapsible preview — never a raw
 * thousand-int dump), and the done reason. Renders an honest "not captured"
 * notice when the body is absent or is not a generation payload.
 */
export function InferenceDetailGeneration({ event }: Readonly<InferenceDetailBodyTabProps>) {
  const [contextOpen, setContextOpen] = useState(false);
  const view = buildGenerationView(event.responseBody);

  if (view === null) {
    return <p className="inference-detail__not-captured">{NOT_CAPTURED_LABEL}</p>;
  }

  return (
    <div className="inference-detail__generation">
      <section className="inference-detail__generation-section">
        <h3 className="inference-detail__generation-label">
          Output{view.outputIsJson ? ' (JSON)' : ''}
        </h3>
        <InferenceDetailCodeBlock raw={view.outputRaw} pretty={view.outputIsJson ? view.output : null} />
      </section>

      <dl className="inference-detail__fields">
        <dt>Context</dt>
        <dd>
          {view.contextTokenCount === null ? (
            UNAVAILABLE_LABEL
          ) : (
            <InferenceDetailContextToggle
              label={`${view.contextTokenCount} tokens`}
              open={contextOpen}
              onToggle={() => setContextOpen((open) => !open)}
            />
          )}
        </dd>
        <dt>Reason</dt>
        <dd>{view.doneReason ?? UNAVAILABLE_LABEL}</dd>
      </dl>

      {contextOpen && view.contextTokenCount !== null ? (
        <pre className="inference-detail__code inference-detail__context-preview">{view.contextPreview}</pre>
      ) : null}
    </div>
  );
}
