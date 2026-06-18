import { useState } from 'react';

import { StatusCodePill } from '../../shared/ui/atoms/status-code-pill';
import { INFERENCE_DETAIL_STATUS_LABELS, INFERENCE_DETAIL_TABS } from './inference-detail.constants';
import { InferenceDetailGeneration } from './inference-detail-generation';
import { InferenceDetailHeaders } from './inference-detail-headers';
import { InferenceDetailOverview } from './inference-detail-overview';
import { InferenceDetailPayload } from './inference-detail-payload';
import { InferenceDetailResponse } from './inference-detail-response';
import { InferenceDetailTiming } from './inference-detail-timing';
import type { InferenceDetailPanelProps, InferenceDetailTabKey } from './inference-detail.types';

/**
 * InferenceDetailPanel is the detail side of the master-detail layout. It renders
 * a DevTools-style tab strip (Overview/Payload/Response/Headers/Timing) for the
 * SELECTED request, or an empty prompt when nothing is selected. Pure
 * presentational: all data arrives precomputed via props.
 */
export function InferenceDetailPanel({ event, overview }: Readonly<InferenceDetailPanelProps>) {
  const [activeTab, setActiveTab] = useState<InferenceDetailTabKey>('overview');

  if (event === null || overview === null) {
    return (
      <section className="inference-detail" aria-label="Inference detail">
        <p className="inference-detail__empty">Select a request to inspect its details.</p>
      </section>
    );
  }

  const statusLabel = INFERENCE_DETAIL_STATUS_LABELS[event.status] ?? 'unknown';

  return (
    <section className="inference-detail" aria-label="Inference detail">
      <header className="inference-detail__header">
        <span className="inference-detail__model">{event.model}</span>
        <span className="inference-detail__status">{statusLabel}</span>
        <StatusCodePill statusCode={event.statusCode ?? null} />
      </header>

      <div className="inference-detail__tabs" role="tablist" aria-label="Request detail tabs">
        {INFERENCE_DETAIL_TABS.map((tab) => (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.key}
            className={`inference-detail__tab${activeTab === tab.key ? ' inference-detail__tab--active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <div className="inference-detail__tabpanel" role="tabpanel">
        {activeTab === 'overview' ? <InferenceDetailOverview viewModel={overview} /> : null}
        {activeTab === 'payload' ? <InferenceDetailPayload event={event} /> : null}
        {activeTab === 'response' ? <InferenceDetailResponse event={event} /> : null}
        {activeTab === 'generation' ? <InferenceDetailGeneration event={event} /> : null}
        {activeTab === 'headers' ? <InferenceDetailHeaders event={event} /> : null}
        {activeTab === 'timing' ? <InferenceDetailTiming event={event} /> : null}
      </div>
    </section>
  );
}
