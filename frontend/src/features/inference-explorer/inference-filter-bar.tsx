import type { InferenceStatusFilter } from '../../shared/store/inference-store.types';
import { INFERENCE_STATUS_FILTER_OPTIONS } from './inference-explorer.constants';
import type { InferenceFilterBarProps } from './inference-explorer.types';

/**
 * InferenceFilterBar renders the free-text search and lifecycle status filter
 * that narrow the request table (R4).
 */
export function InferenceFilterBar({ query, statusFilter, onQueryChange, onStatusFilterChange }: Readonly<InferenceFilterBarProps>) {
  return (
    <div className="inference-filter-bar">
      <input
        className="inference-filter-bar__search"
        type="search"
        placeholder="Filter by model or endpoint…"
        value={query}
        aria-label="Filter requests"
        onChange={(event) => onQueryChange(event.target.value)}
      />
      <select
        className="inference-filter-bar__status"
        value={String(statusFilter)}
        aria-label="Filter by status"
        onChange={(event) =>
          onStatusFilterChange(
            event.target.value === 'all' ? 'all' : (Number(event.target.value) as InferenceStatusFilter),
          )
        }
      >
        {INFERENCE_STATUS_FILTER_OPTIONS.map((option) => (
          <option key={String(option.value)} value={String(option.value)}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}
