import type { KeyboardEvent } from 'react';

import { formatClockTime } from '../../shared/helpers/formatters.helpers';
import { deriveDisplayTiming } from '../../shared/helpers/live-timing.helpers';
import { LatencyPill } from '../../shared/ui/atoms/latency-pill';
import { StatusCodePill } from '../../shared/ui/atoms/status-code-pill';
import { TokenRateBadge } from '../../shared/ui/atoms/token-rate-badge';
import { WaterfallBar } from '../../shared/ui/atoms/waterfall-bar';
import { INFERENCE_STATUS_LABELS } from './inference-explorer.constants';
import type { InferenceTableRowProps } from './inference-explorer.types';

// All row/cell elements use ARIA roles instead of native <tr>/<td> because the
// virtualizer requires position:absolute on rows, which <tr> does not support.

/**
 * InferenceTableRow renders a single request as a dense, selectable table row.
 * Pure presentational: derives display values from the event and atoms; the
 * absolute position comes from the virtualizer via `style`. The latency and
 * waterfall reflect live elapsed wall-clock while the request is in progress
 * (nowMS, supplied by the table); tok/s stays unavailable until completion.
 */
export function InferenceTableRow({ event, rowId, isSelected, maxLatencyMS, nowMS, style, onSelect }: Readonly<InferenceTableRowProps>) {
  const statusLabel = INFERENCE_STATUS_LABELS[event.status] ?? 'unknown';
  const perSec = event.tokens?.perSec ?? null;
  const { loadMS, evalMS, totalMS: latencyMS } = deriveDisplayTiming(event, nowMS);

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onSelect(rowId);
    }
  }

  const rowClass = `inference-table__row${isSelected ? ' inference-table__row--selected' : ''}`;

  /* eslint-disable react-doctor/prefer-tag-over-role -- virtualizer requires
     position:absolute on rows, which <tr>/<td> does not support. */
  return (
    <div
      role="row"
      aria-selected={isSelected}
      className={rowClass}
      style={style}
      tabIndex={0}
      onClick={() => onSelect(rowId)}
      onKeyDown={handleKeyDown}
    >
      <div className="inference-table__cell inference-table__cell--model" title={event.model} role="cell">{event.model}</div>
      <div className="inference-table__cell inference-table__cell--endpoint" title={event.endpoint} role="cell">{event.endpoint}</div>
      <div className="inference-table__cell inference-table__cell--method" role="cell">{event.method}</div>
      <div className="inference-table__cell inference-table__cell--status" role="cell">{statusLabel}</div>
      <div className="inference-table__cell inference-table__cell--code" role="cell"><StatusCodePill statusCode={event.statusCode ?? null} /></div>
      <div className="inference-table__cell inference-table__cell--rate" role="cell"><TokenRateBadge perSec={perSec} /></div>
      <div className="inference-table__cell inference-table__cell--latency" role="cell"><LatencyPill latencyMS={latencyMS} /></div>
      <div className="inference-table__cell inference-table__cell--time" role="cell">{formatClockTime(event.at)}</div>
      <div className="inference-table__cell inference-table__cell--waterfall" role="cell">
        <WaterfallBar loadMS={loadMS} evalMS={evalMS} totalMS={latencyMS} maxMS={maxLatencyMS} />
      </div>
    </div>
  );
  /* eslint-enable react-doctor/prefer-tag-over-role */
}
