package sqlite

import (
	"context"
	"fmt"
	"time"
)

// Prune enforces bounded retention (spec "Bounded retention" requirement) by
// deleting rows older than maxAge (when maxAge > 0) and, independently,
// trimming down to the maxCount most recent rows by At (when maxCount > 0).
// A zero value for either parameter disables that cap. Both caps can be
// applied together; each runs in its own statement so a disabled cap never
// touches the table.
func (s *Store) Prune(ctx context.Context, maxCount int, maxAge time.Duration) error {
	if maxAge > 0 {
		cutoff := time.Now().Add(-maxAge).UnixNano()
		if _, err := s.db.ExecContext(ctx, `DELETE FROM inferences WHERE at < ?`, cutoff); err != nil {
			return fmt.Errorf("sqlite: prune by age: %w", err)
		}
	}

	if maxCount > 0 {
		const stmt = `
DELETE FROM inferences WHERE id IN (
	SELECT id FROM inferences ORDER BY at DESC LIMIT -1 OFFSET ?
)`
		if _, err := s.db.ExecContext(ctx, stmt, maxCount); err != nil {
			return fmt.Errorf("sqlite: prune by count: %w", err)
		}
	}

	return nil
}
