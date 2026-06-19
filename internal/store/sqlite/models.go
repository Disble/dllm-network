package sqlite

import (
	"context"
	"fmt"
)

// Models implements store.InferenceReader.Models: the distinct model names
// observed in stored inferences (spec "List distinct models"). Returns an
// empty, non-nil slice when the store has no rows. Order is whatever SQLite
// returns from DISTINCT — callers needing a stable order must sort.
func (s *Store) Models(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT model FROM inferences`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: models: %w", err)
	}
	defer rows.Close()

	models := make([]string, 0)
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("sqlite: models scan: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: models rows: %w", err)
	}

	return models, nil
}
