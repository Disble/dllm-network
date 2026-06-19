// Package sqlite implements the durable, cross-process SQLite-backed
// inference repository (design D1/D2/D5). It is the sole adapter that knows
// about database/sql and the modernc.org/sqlite pure-Go driver — the domain
// type (internal/telemetry/inference) and the live in-memory dashboard
// projection (internal/store.Recent) stay free of any storage concern.
//
// Pure-Go only: modernc.org/sqlite requires no CGO, which is required to
// keep the WinDivert Windows build and cross-compile working.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for database/sql

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// Compile-time assertions: Store implements both segregated ports from
// design D5. Query/Get/Stats/Models (the reader surface) are implemented in
// query.go, stats.go, and models.go (slice 3); Save (the writer surface) is
// implemented below (slice 1).
var (
	_ store.InferenceWriter = (*Store)(nil)
	_ store.InferenceReader = (*Store)(nil)
)

// writerDSNOptions are the database/sql DSN query parameters applied to the
// writer connection: WAL journal mode for multi-process readability while a
// writer holds the file open, a busy_timeout to absorb WAL checkpoint
// contention without surfacing SQLITE_BUSY, and immediate transaction
// locking so writers fail fast instead of silently upgrading mid-transaction.
//
// modernc.org/sqlite (unlike mattn/go-sqlite3) does not recognize bare
// "_journal_mode"/"_busy_timeout" query params — pragmas must be set via
// repeated "_pragma=name(value)" parameters, applied as the connection opens.
const writerDSNOptions = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"

// readOnlyDSNOptions are applied to the stdio sidecar's connection: WAL mode
// so it can read while the writer (GUI) holds the file open, the same
// busy_timeout to ride out checkpoint contention, mode=ro as a hint, and
// _pragma=query_only(true) as the connection-level enforcement that actually
// rejects writes.
//
// mode=ro ALONE IS NOT ENOUGH: it is a SQLite URI-query parameter that
// modernc.org/sqlite (and SQLite generally) only honors when the DSN itself
// is in "file:" URI form (e.g. "file:"+path+"?mode=ro&..."). This DSN is
// appended to a bare path (path+readOnlyDSNOptions, not a file: URI), so
// mode=ro is silently ignored and the connection would otherwise accept
// writes — confirmed by a direct probe during Slice 5 apply and locked in by
// TestOpenReadOnly_RejectsWrites. _pragma=query_only(true) IS honored on a
// bare-path DSN (pragmas apply via repeated "_pragma=name(value)" params
// regardless of DSN form, same as writerDSNOptions above) and makes SQLite
// itself reject every write statement at the connection level, independent
// of the URI-only mode=ro hint. mode=ro is kept anyway — harmless, and
// correct if a future caller switches this DSN to file: URI form.
//
// query_only does NOT prevent file creation on a missing database file; that
// case is handled by the stdio sidecar's pre-open os.Stat existence check
// (cmd/ollama-telemetry-mcp/run.go), deliberately not by this DSN.
const readOnlyDSNOptions = "?mode=ro&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=query_only(true)"

// Store is the SQLite-backed implementation of the (forthcoming)
// InferenceWriter/InferenceReader ports. Slice 1 implements only the writer
// surface (Save) plus Get, which both the writer and later the read API
// depend on.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path in
// read-write WAL mode and applies any pending migrations. Open is intended
// for the single writer (GUI app) — see OpenReadOnly for the stdio sidecar's
// read-only connection.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+writerDSNOptions)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}

	// modernc.org/sqlite is not safe for concurrent writer connections from
	// the same process to the same file; the writer side of this store is a
	// single logical connection by design (one GUI process, one Store).
	db.SetMaxOpenConns(1)

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// OpenReadOnly opens the SQLite database at path as a read-only connection
// in WAL mode, without attempting any migration. It is intended for the
// stdio sidecar, which must never write and must coexist with the GUI app's
// writer connection on the same file. If the database's schema is newer
// than this binary knows (PRAGMA user_version > schemaVersion), it fails
// fast with ErrSchemaTooNew instead of silently serving against an unknown
// schema.
func OpenReadOnly(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+readOnlyDSNOptions)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open read-only %s: %w", path, err)
	}

	if err := checkSchemaCompatible(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Save persists infs as a single batched transaction. Each row is
// inserted/replaced by ID so re-publishing the same completed inference
// (e.g. after a retry) is idempotent.
func (s *Store) Save(ctx context.Context, infs []inference.Inference) error {
	if len(infs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin save: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("sqlite: prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, inf := range infs {
		if err := saveOne(ctx, stmt, inf); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit save: %w", err)
	}
	return nil
}

const insertSQL = `
INSERT INTO inferences (
	id, at, model, endpoint, method, status, status_code, streaming,
	prompt_size, prompt_eval_count, eval_count, eval_duration_ns,
	total_duration_ns, load_duration_ns, per_sec, latency_ms, detail
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	at = excluded.at, model = excluded.model, endpoint = excluded.endpoint,
	method = excluded.method, status = excluded.status,
	status_code = excluded.status_code, streaming = excluded.streaming,
	prompt_size = excluded.prompt_size,
	prompt_eval_count = excluded.prompt_eval_count,
	eval_count = excluded.eval_count,
	eval_duration_ns = excluded.eval_duration_ns,
	total_duration_ns = excluded.total_duration_ns,
	load_duration_ns = excluded.load_duration_ns,
	per_sec = excluded.per_sec, latency_ms = excluded.latency_ms,
	detail = excluded.detail`

func saveOne(ctx context.Context, stmt *sql.Stmt, inf inference.Inference) error {
	detailJSON, err := marshalDetail(inf)
	if err != nil {
		return err
	}

	tokens := inf.Tokens
	_, err = stmt.ExecContext(ctx,
		inf.ID, inf.At.UnixNano(), inf.Model, inf.Endpoint, inf.Method,
		int(inf.Status), inf.StatusCode, boolToInt(inf.Streaming), inf.PromptSize,
		nullableInt(tokens != nil, tokensPromptEvalCount(tokens)),
		nullableInt(tokens != nil, tokensEvalCount(tokens)),
		nullableInt64(tokens != nil, tokensEvalDurationNS(tokens)),
		nullableInt64(tokens != nil, tokensTotalDurationNS(tokens)),
		nullableInt64(tokens != nil, tokensLoadDurationNS(tokens)),
		nullableFloat(tokens != nil, tokensPerSec(tokens)),
		nullableFloat(tokens != nil, tokensLatencyMS(tokens)),
		detailJSON,
	)
	if err != nil {
		return fmt.Errorf("sqlite: insert inference %s: %w", inf.ID, err)
	}
	return nil
}

// Get fetches one full record by ID, including bodies and headers. Returns
// ok=false (no error) when the ID is unknown.
func (s *Store) Get(ctx context.Context, id string) (inference.Inference, bool, error) {
	row := s.db.QueryRowContext(ctx, selectByIDSQL, id)

	inf, err := scanInference(row)
	if err == sql.ErrNoRows {
		return inference.Inference{}, false, nil
	}
	if err != nil {
		return inference.Inference{}, false, fmt.Errorf("sqlite: get %s: %w", id, err)
	}
	return inf, true, nil
}

const selectByIDSQL = `
SELECT id, at, model, endpoint, method, status, status_code, streaming,
	prompt_size, detail
FROM inferences WHERE id = ?`
