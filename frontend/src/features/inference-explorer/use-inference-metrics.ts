import { useEffect, useMemo } from 'react';

import type { DashboardSnapshotSource } from '../../infrastructure/dashboard-snapshot-source';
import { formatTimestamp } from '../../shared/helpers/formatters.helpers';
import { connectInferenceStore, useInferenceStore } from '../../shared/store/inference-store';
import { computeAggregates } from '../../shared/store/inference-store.helpers';
import type { UseInferenceMetricsResult } from './inference-explorer.types';

/**
 * useInferenceMetrics derives the top KPI strip cells from the shared store:
 * request count, average tok/s, p50/p95 latency, total eval count, and the last
 * updated timestamp. Unavailable metrics render an honest em-dash.
 */
export function useInferenceMetrics(source?: DashboardSnapshotSource): UseInferenceMetricsResult {
  const events = useInferenceStore((state) => state.events);

  const items = useMemo(() => {
    const aggregates = computeAggregates(events);
    return [
      { label: 'Requests', value: String(aggregates.count), caption: 'Total' },
      {
        label: 'Avg tok/s',
        value: aggregates.avgPerSec === null ? '—' : `${aggregates.avgPerSec.toFixed(1)} tok/s`,
        caption: 'Average',
      },
      {
        label: 'P50 latency',
        value: aggregates.p50LatencyMS === null ? '—' : `${Math.round(aggregates.p50LatencyMS)} ms`,
        caption: 'Median',
      },
      {
        label: 'P95 latency',
        value: aggregates.p95LatencyMS === null ? '—' : `${Math.round(aggregates.p95LatencyMS)} ms`,
        caption: '95th Percentile',
      },
      { label: 'Eval count', value: String(aggregates.totalEvalCount), caption: 'Total' },
      {
        label: 'Timestamp',
        value: aggregates.lastUpdated === '' ? '—' : formatTimestamp(aggregates.lastUpdated),
        caption: 'Last Updated',
      },
    ];
  }, [events]);

  useEffect(() => {
    connectInferenceStore(source);
  }, [source]);

  return { items };
}
