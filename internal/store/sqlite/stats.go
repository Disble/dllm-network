package sqlite

import (
	"context"
	"fmt"
	"math"
	"sort"

	"dllm-network/internal/store"
)

// Stats implements store.InferenceReader.Stats: tokens/sec and latency
// percentiles (p50/p95, nearest-rank method) over the rows matching filter
// that have non-NULL per_sec/latency_ms (i.e. Tokens != nil at write time),
// plus a per-model row count over ALL matching rows (not just those with
// token stats) so ByModel counts always sum to filter's total match count.
func (s *Store) Stats(ctx context.Context, filter store.Filter) (store.Stats, error) {
	clause, args := whereClause(filter)

	count, err := s.countMatching(ctx, clause, args)
	if err != nil {
		return store.Stats{}, err
	}

	perSecs, latencies, err := s.fetchTokenMetrics(ctx, clause, args)
	if err != nil {
		return store.Stats{}, err
	}

	byModel, err := s.countByModel(ctx, clause, args)
	if err != nil {
		return store.Stats{}, err
	}

	return store.Stats{
		Count:        count,
		PerSecP50:    percentile(perSecs, 0.50),
		PerSecP95:    percentile(perSecs, 0.95),
		LatencyMSP50: percentile(latencies, 0.50),
		LatencyMSP95: percentile(latencies, 0.95),
		ByModel:      byModel,
	}, nil
}

func (s *Store) countMatching(ctx context.Context, whereClause string, args []any) (int, error) {
	if err := validateClauseColumns(whereClause); err != nil {
		return 0, err
	}
	var count int
	q := "SELECT COUNT(*) FROM inferences" + whereClause
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: stats count: %w", err)
	}
	return count, nil
}

// fetchTokenMetrics returns the per_sec and latency_ms values for rows
// matching whereClause that have non-NULL per_sec (i.e. were saved with
// Tokens != nil). Rows without token stats are excluded from the
// percentile inputs but still counted in countMatching/countByModel.
func (s *Store) fetchTokenMetrics(ctx context.Context, whereClause string, args []any) ([]float64, []float64, error) {
	if err := validateClauseColumns(whereClause); err != nil {
		return nil, nil, err
	}
	cond := "per_sec IS NOT NULL"
	q := "SELECT per_sec, latency_ms FROM inferences"
	if whereClause == "" {
		q += " WHERE " + cond
	} else {
		q += whereClause + " AND " + cond
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("sqlite: stats token metrics: %w", err)
	}
	defer rows.Close()

	var perSecs, latencies []float64
	for rows.Next() {
		var perSec, latencyMS float64
		if err := rows.Scan(&perSec, &latencyMS); err != nil {
			return nil, nil, fmt.Errorf("sqlite: stats token metrics scan: %w", err)
		}
		perSecs = append(perSecs, perSec)
		latencies = append(latencies, latencyMS)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("sqlite: stats token metrics rows: %w", err)
	}

	return perSecs, latencies, nil
}

func (s *Store) countByModel(ctx context.Context, whereClause string, args []any) ([]store.ModelStats, error) {
	if err := validateClauseColumns(whereClause); err != nil {
		return nil, err
	}
	q := "SELECT model, COUNT(*) FROM inferences" + whereClause + " GROUP BY model"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: stats by model: %w", err)
	}
	defer rows.Close()

	byModel := make([]store.ModelStats, 0)
	for rows.Next() {
		var ms store.ModelStats
		if err := rows.Scan(&ms.Model, &ms.Count); err != nil {
			return nil, fmt.Errorf("sqlite: stats by model scan: %w", err)
		}
		byModel = append(byModel, ms)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: stats by model rows: %w", err)
	}

	return byModel, nil
}

// percentile computes p (0..1) over values using the nearest-rank method:
// sort ascending, take the value at index ceil(p*n)-1. Returns 0 for an
// empty input (spec: "Stats over empty dataset" reports zero, not NaN or a
// panic).
func percentile(values []float64, p float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	rank := int(math.Ceil(p * float64(n)))
	idx := rank - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}
