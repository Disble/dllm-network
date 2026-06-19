package sqlite

import (
	"context"
	"fmt"
	"strings"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// Query implements store.InferenceReader.Query: a filtered list ordered
// most-recent-first (by at DESC), capped at filter.Limit when > 0. Returns
// an empty, non-nil slice (not an error) when nothing matches (spec "No
// matches returns empty, not error").
func (s *Store) Query(ctx context.Context, filter store.Filter) ([]inference.Inference, error) {
	clause, args := whereClause(filter)

	q := fmt.Sprintf(`SELECT id, at, model, endpoint, method, status, status_code,
		streaming, prompt_size, detail FROM inferences%s ORDER BY at DESC`, clause)
	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query: %w", err)
	}
	defer rows.Close()

	results := make([]inference.Inference, 0)
	for rows.Next() {
		inf, err := scanInference(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: query scan: %w", err)
		}
		results = append(results, inf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: query rows: %w", err)
	}

	return results, nil
}

// whereClause renders f as a SQL "WHERE ..." clause (or "" when no field is
// set) plus the matching positional args, shared by Query and Stats so the
// filter-to-SQL mapping lives in exactly one place.
func whereClause(f store.Filter) (string, []any) {
	var conds []string
	var args []any

	if f.Model != "" {
		conds = append(conds, "model = ?")
		args = append(args, f.Model)
	}
	if f.Endpoint != "" {
		conds = append(conds, "endpoint = ?")
		args = append(args, f.Endpoint)
	}
	if f.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, int(*f.Status))
	}
	if !f.Since.IsZero() {
		conds = append(conds, "at >= ?")
		args = append(args, f.Since.UnixNano())
	}
	if !f.Until.IsZero() {
		conds = append(conds, "at < ?")
		args = append(args, f.Until.UnixNano())
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
