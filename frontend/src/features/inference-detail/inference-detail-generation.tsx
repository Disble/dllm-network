import { useState } from 'react';

import { NOT_CAPTURED_LABEL, UNAVAILABLE_LABEL } from './inference-detail.constants';
import { InferenceDetailCodeBlock } from './inference-detail-code-block';
import { InferenceDetailContextToggle } from './inference-detail-context-toggle';
import { buildGenerationView } from './inference-detail-generation-view-model.helpers';
import type { InferenceDetailBodyTabProps } from './inference-detail.types';

/**
 * InferenceDetailGeneration renders the LLM-aware view of a model response (the
 * Generation tab): the pretty-printed output, the reasoning trace when present,
 * a compact context token summary (count + collapsible preview — never a raw
 * thousand-int dump), and the finish reason. The content is normalized at the
 * backend boundary (event.generation), so this tab works identically for
 * Ollama-native and OpenAI streams. Renders an honest "not captured" notice when
 * no generation payload exists.
 */
export function InferenceDetailGeneration({ event }: Readonly<InferenceDetailBodyTabProps>) {
  const [contextOpen, setContextOpen] = useState(false);
  const view = buildGenerationView(event.generation);

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

      {view.reasoning !== '' ? (
        <section className="inference-detail__generation-section">
          <h3 className="inference-detail__generation-label">Reasoning</h3>
          <InferenceDetailCodeBlock raw={view.reasoning} />
        </section>
      ) : null}

      {view.toolCalls.map((call, index) => (
        <section className="inference-detail__generation-section" key={`${call.name}-${index}`}>
          <h3 className="inference-detail__generation-label">
            Tool call: <span className="inference-detail__tool-name">{call.name}</span>
          </h3>
          <InferenceDetailCodeBlock
            raw={call.argumentsRaw}
            pretty={call.argumentsIsJson ? call.arguments : null}
          />
        </section>
      ))}

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
