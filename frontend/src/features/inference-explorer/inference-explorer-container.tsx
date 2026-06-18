import { InferenceFilterBar } from './inference-filter-bar';
import { InferenceTable } from './inference-table';
import { useInferenceExplorer } from './use-inference-explorer';
import type { InferenceExplorerContainerProps } from './inference-explorer.types';

/**
 * InferenceExplorerContainer is the master side of the DevTools-Network layout:
 * a titled toolbar with search + status filters and the virtualized request
 * table. Summary metrics live in the top KPI strip (InferenceKpiContainer); this
 * panel focuses on the request list and drives the selection the detail reads.
 */
export function InferenceExplorerContainer({ source }: Readonly<InferenceExplorerContainerProps>) {
  const { rows, selectedId, query, statusFilter, captureUnavailable, captureNote, onQueryChange, onStatusFilterChange, onSelect } =
    useInferenceExplorer(source);

  return (
    <section className="inference-explorer" aria-label="Inference explorer">
      <header className="inference-explorer__toolbar">
        <h2 className="inference-explorer__title">Requests</h2>
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
