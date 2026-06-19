package sqlite

import (
	"database/sql"
	"fmt"
)

// schemaVersion is the schema version this build of the code knows about —
// equal to len(migrations). A writer (GUI) applies pending migrations up to
// this version on Open. A read-only sidecar compares its compiled-in
// schemaVersion against the database's PRAGMA user_version and fails fast if
// the database is newer (see ErrSchemaTooNew), guarding against version skew
// between an updated GUI and a stale sidecar binary.
var schemaVersion = len(migrations)

// ErrSchemaTooNew is returned by Open when the database's PRAGMA user_version
// is ahead of this build's known schemaVersion and the connection cannot
// migrate (read-only). The caller (typically the stdio sidecar) should
// surface this as "GUI newer than sidecar — update sidecar".
var ErrSchemaTooNew = fmt.Errorf("sqlite: database schema is newer than this binary knows; update this binary")

// applyMigrations brings db from its current PRAGMA user_version up to
// schemaVersion by executing each pending migration in migrations, in order,
// each inside its own transaction. It is a no-op when already current.
func applyMigrations(db *sql.DB) error {
	current, err := userVersion(db)
	if err != nil {
		return fmt.Errorf("sqlite: read user_version: %w", err)
	}

	if current > schemaVersion {
		return ErrSchemaTooNew
	}

	for v := current; v < schemaVersion; v++ {
		stmt := migrations[v]
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("sqlite: begin migration %d: %w", v+1, err)
		}

		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("sqlite: apply migration %d: %w", v+1, err)
		}

		if err := setUserVersion(tx, v+1); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("sqlite: set user_version to %d: %w", v+1, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("sqlite: commit migration %d: %w", v+1, err)
		}
	}

	return nil
}

// checkSchemaCompatible reports ErrSchemaTooNew when the database's
// user_version exceeds this build's schemaVersion, without attempting to
// migrate. Used by read-only connections (the stdio sidecar) that must never
// write but still need to fail fast on version skew.
func checkSchemaCompatible(db *sql.DB) error {
	current, err := userVersion(db)
	if err != nil {
		return fmt.Errorf("sqlite: read user_version: %w", err)
	}
	if current > schemaVersion {
		return ErrSchemaTooNew
	}
	return nil
}

func userVersion(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func setUserVersion(tx *sql.Tx, v int) error {
	// PRAGMA does not support bound parameters; v is an internal int (slice
	// index + 1), never user input, so direct interpolation is safe here.
	_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
	return err
}
