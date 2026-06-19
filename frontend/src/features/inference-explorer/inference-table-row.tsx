import { formatClockTime } from '../../shared/helpers/formatters.helpers';
import { deriveDisplayTiming } from '../../shared/helpers/live-timing.helpers';
import { LatencyPill } from '../../shared/ui/atoms/latency-pill';
import { StatusCodePill } from '../../shared/ui/atoms/status-code-pill';
import { TokenRateBadge } from '../../shared/ui/atoms/token-rate-badge';
import { WaterfallBar } from '../../shared/ui/atoms/waterfall-bar';
import { INFERENCE_STATUS_LABELS } from './inference-explorer.constants';
import type { InferenceTableRowProps } from './inference-explorer.types';

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

  return (
    <button
      type="button"
      role="row"
      aria-selected={isSelected}
      className={`inference-table__row${isSelected ? ' inference-table__row--selected' : ''}`}
      style={style}
      onClick={() => onSelect(rowId)}
    >
      <span className="inference-table__cell inference-table__cell--model" title={event.model}>{event.model}</span>
      <span className="inference-table__cell inference-table__cell--endpoint" title={event.endpoint}>{event.endpoint}</span>
      <span className="inference-table__cell inference-table__cell--method">{event.method}</span>
      <span className="inference-table__cell inference-table__cell--status">{statusLabel}</span>
      <span className="inference-table__cell inference-table__cell--code"><StatusCodePill statusCode={event.statusCode ?? null} /></span>
      <span className="inference-table__cell inference-table__cell--rate"><TokenRateBadge perSec={perSec} /></span>
      <span className="inference-table__cell inference-table__cell--latency"><LatencyPill latencyMS={latencyMS} /></span>
      <span className="inference-table__cell inference-table__cell--time">{formatClockTime(event.at)}</span>
      <span className="inference-table__cell inference-table__cell--waterfall">
        <WaterfallBar loadMS={loadMS} evalMS={evalMS} totalMS={latencyMS} maxMS={maxLatencyMS} />
      </span>
    </button>
  );
}
