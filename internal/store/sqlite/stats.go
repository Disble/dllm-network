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
// statsFilterArgs returns positional arguments for the static stats WHERE
// clause. Each optional condition consumes two arguments: a skip flag (1 to
// ignore the condition, 0 to enforce it) and the comparison value. When a
// filter field is unset the corresponding condition becomes "1 = 1" and the
// value is ignored, so the query text never changes based on user input.
func statsFilterArgs(f store.Filter) []any {
	status := 0
	if f.Status != nil {
		status = int(*f.Status)
	}
	return []any{
		boolInt(f.Model == ""), f.Model,
		boolInt(f.Endpoint == ""), f.Endpoint,
		boolInt(f.Status == nil), status,
		boolInt(f.Since.IsZero()), f.Since.UnixNano(),
		boolInt(f.Until.IsZero()), f.Until.UnixNano(),
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// stats query constants use fully static SQL with parameter-driven skip flags so
// the query text never changes based on user input.
const (
	statsCountSQL = `SELECT COUNT(*) FROM inferences WHERE (1 = ? OR model = ?) AND (1 = ? OR endpoint = ?) AND (1 = ? OR status = ?) AND (1 = ? OR at >= ?) AND (1 = ? OR at < ?)`
	statsTokenSQL = `SELECT per_sec, latency_ms FROM inferences WHERE (1 = ? OR model = ?) AND (1 = ? OR endpoint = ?) AND (1 = ? OR status = ?) AND (1 = ? OR at >= ?) AND (1 = ? OR at < ?) AND per_sec IS NOT NULL`
	statsModelSQL = `SELECT model, COUNT(*) FROM inferences WHERE (1 = ? OR model = ?) AND (1 = ? OR endpoint = ?) AND (1 = ? OR status = ?) AND (1 = ? OR at >= ?) AND (1 = ? OR at < ?) GROUP BY model`
)

func (s *Store) Stats(ctx context.Context, filter store.Filter) (store.Stats, error) {
	args := statsFilterArgs(filter)

	count, err := s.countMatching(ctx, args)
	if err != nil {
		return store.Stats{}, err
	}

	perSecs, latencies, err := s.fetchTokenMetrics(ctx, args)
	if err != nil {
		return store.Stats{}, err
	}

	byModel, err := s.countByModel(ctx, args)
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

func (s *Store) countMatching(ctx context.Context, args []any) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, statsCountSQL, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: stats count: %w", err)
	}
	return count, nil
}

// fetchTokenMetrics returns the per_sec and latency_ms values for rows
// matching the filter that have non-NULL per_sec (i.e. were saved with
// Tokens != nil). Rows without token stats are excluded from the
// percentile inputs but still counted in countMatching/countByModel.
func (s *Store) fetchTokenMetrics(ctx context.Context, args []any) ([]float64, []float64, error) {
	rows, err := s.db.QueryContext(ctx, statsTokenSQL, args...)
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

func (s *Store) countByModel(ctx context.Context, args []any) ([]store.ModelStats, error) {
	rows, err := s.db.QueryContext(ctx, statsModelSQL, args...)
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
