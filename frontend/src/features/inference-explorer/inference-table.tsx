import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';

import { deriveEventId } from '../../shared/store/inference-store.helpers';
import { INFERENCE_ROW_HEIGHT, INFERENCE_TABLE_OVERSCAN } from './inference-explorer.constants';
import { InferenceTableRow } from './inference-table-row';
import type { InferenceTableProps } from './inference-explorer.types';

/**
 * InferenceTable renders the virtualized, scannable request table. Only the rows
 * intersecting the viewport (plus overscan) are mounted, so the table stays
 * smooth from tens to thousands of captured requests (R1).
 */
export function InferenceTable({ rows, selectedId, onSelect }: Readonly<InferenceTableProps>) {
  // eslint-disable-next-line no-undef -- DOM lib type; the flat config only declares document/window as globals.
  const scrollRef = useRef<HTMLDivElement>(null);

  // Waterfall scale reference: the slowest request in the visible set. Guarded at
  // >= 1 so a single zero-latency row never divides by zero.
  const maxLatencyMS = Math.max(1, ...rows.map((row) => row.tokens?.latencyMS ?? 0));

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => INFERENCE_ROW_HEIGHT,
    overscan: INFERENCE_TABLE_OVERSCAN,
  });

  // The virtualizer absolutely-positions each row as a <button> inside the
  // scroll area. Native <table>/<tbody>/<tr> elements cannot host absolutely
  // positioned children without breaking both layout and virtualization, so we
  // keep the explicit ARIA table roles on the equivalent div containers.
  return (
    // eslint-disable-next-line react-doctor/prefer-tag-over-role
    <div className="inference-table" role="table" aria-label="Captured inference requests">
      {/* eslint-disable-next-line react-doctor/prefer-tag-over-role -- virtualized table header rendered as a div row; see explanatory comment above. */}
      <div className="inference-table__head" role="row">
        <span className="inference-table__cell inference-table__cell--model">Model</span>
        <span className="inference-table__cell inference-table__cell--endpoint">Endpoint</span>
        <span className="inference-table__cell inference-table__cell--method">Method</span>
        <span className="inference-table__cell inference-table__cell--status">Status</span>
        <span className="inference-table__cell inference-table__cell--code">Code</span>
        <span className="inference-table__cell inference-table__cell--rate">Tok/s</span>
        <span className="inference-table__cell inference-table__cell--latency">Latency</span>
        <span className="inference-table__cell inference-table__cell--time">Time</span>
        <span className="inference-table__cell inference-table__cell--waterfall">Waterfall</span>
      </div>

      {/* eslint-disable-next-line react-doctor/prefer-tag-over-role -- virtualized scroll body rendered as a div rowgroup; see explanatory comment above. */}
      <div ref={scrollRef} className="inference-table__scroll" role="rowgroup">
        {rows.length === 0 ? (
          <p className="inference-table__empty">No inference requests captured yet.</p>
        ) : (
          <div className="inference-table__viewport" style={{ height: `${virtualizer.getTotalSize()}px` }}>
            {virtualizer.getVirtualItems().map((item) => {
              const event = rows[item.index];
              const rowId = deriveEventId(event);
              return (
                <InferenceTableRow
                  key={rowId}
                  event={event}
                  rowId={rowId}
                  isSelected={rowId === selectedId}
                  maxLatencyMS={maxLatencyMS}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${INFERENCE_ROW_HEIGHT}px`,
                    transform: `translateY(${item.start}px)`,
                  }}
                  onSelect={onSelect}
                />
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
