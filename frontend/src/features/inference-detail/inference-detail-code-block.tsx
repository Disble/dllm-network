import { useState } from 'react';

import { COPIED_RESET_MS } from './inference-detail.constants';
import type { InferenceDetailCodeBlockProps } from './inference-detail.types';

/**
 * InferenceDetailCodeBlock is the shared code viewer for Payload, Response and
 * the Generation output. It shows `raw` verbatim, offers a one-click Pretty/Raw
 * toggle when a `pretty` form is supplied (Pretty default), and always exposes a
 * Copy button that copies the verbatim `raw` text regardless of the current view.
 */
export function InferenceDetailCodeBlock({ raw, pretty, truncated }: Readonly<InferenceDetailCodeBlockProps>) {
  const [view, setView] = useState<'pretty' | 'raw'>('pretty');
  const [copied, setCopied] = useState(false);

  const hasPretty = pretty !== undefined && pretty !== null;
  const shown = hasPretty && view === 'pretty' ? pretty : raw;

  const handleCopy = () => {
    void window.navigator.clipboard?.writeText(raw);
    setCopied(true);
    window.setTimeout(() => setCopied(false), COPIED_RESET_MS);
  };

  return (
    <div className="inference-detail__body">
      <div className="inference-detail__code-toolbar">
        {hasPretty ? (
          <div className="inference-detail__view-toggle">
            <button
              type="button"
              className={`inference-detail__view-option${view === 'pretty' ? ' inference-detail__view-option--active' : ''}`}
              aria-pressed={view === 'pretty'}
              onClick={() => setView('pretty')}
            >
              Pretty
            </button>
            <button
              type="button"
              className={`inference-detail__view-option${view === 'raw' ? ' inference-detail__view-option--active' : ''}`}
              aria-pressed={view === 'raw'}
              onClick={() => setView('raw')}
            >
              Raw
            </button>
          </div>
        ) : null}
        <button type="button" className="inference-detail__copy" onClick={handleCopy}>
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <pre className="inference-detail__code">{shown}</pre>
      {truncated ? <p className="inference-detail__truncated">Truncated at capture limit.</p> : null}
    </div>
  );
}
