import { InferenceAggregates } from './inference-aggregates';
import { InferenceFilterBar } from './inference-filter-bar';
import { InferenceTable } from './inference-table';
import { useInferenceExplorer } from './use-inference-explorer';
import type { InferenceExplorerContainerProps } from './inference-explorer.types';

/**
 * InferenceExplorerContainer is the master side of the DevTools-Network layout:
 * a summary header, search + status filters, and the virtualized request table.
 * It subscribes to the shared inference store via the explorer hook and drives
 * selection that the detail panel reads back.
 */
export function InferenceExplorerContainer({ source }: Readonly<InferenceExplorerContainerProps>) {
  const {
    rows,
    selectedId,
    query,
    statusFilter,
    aggregates,
    captureUnavailable,
    captureNote,
    onQueryChange,
    onStatusFilterChange,
    onSelect,
  } = useInferenceExplorer(source);

  return (
    <section className="inference-explorer" aria-label="Inference explorer">
      <header className="inference-explorer__toolbar">
        <InferenceAggregates aggregates={aggregates} />
        <InferenceFilterBar
          query={query}
          statusFilter={statusFilter}
          onQueryChange={onQueryChange}
          onStatusFilterChange={onStatusFilterChange}
        />
      </header>
      {captureUnavailable ? (
        <output className="inference-explorer__banner">
          Live inference capture is unavailable. {captureNote}
        </output>
      ) : null}
      <InferenceTable rows={rows} selectedId={selectedId} onSelect={onSelect} />
    </section>
  );
}
