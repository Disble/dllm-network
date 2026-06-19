package sqlite

// migrations is an ordered list of forward-only schema migrations. Each
// entry's index+1 is its target schema version, gated by PRAGMA user_version
// (see migrate.go). To add a migration, append a new entry — never edit or
// reorder existing ones, since the version number is derived from position.
var migrations = []string{
	// Migration 1: initial schema. Hybrid layout — queryable/filterable fields
	// are scalar columns (model, endpoint, status, at, statusCode, perSec,
	// latencyMS, evalCount, promptEvalCount, promptSize, streaming); heavy or
	// optional fields (request/response bodies + truncation flags, headers,
	// the full TokenStats) are packed into one JSON "detail" blob. Token-stat
	// columns are NULLable: NULL means inference.TokenStats was nil (status
	// not applicable), never coerced to zero — see detail.go / domain
	// contract in internal/telemetry/inference/types.go.
	`CREATE TABLE inferences (
		id                TEXT PRIMARY KEY,
		at                INTEGER NOT NULL,
		model             TEXT NOT NULL,
		endpoint          TEXT NOT NULL,
		method            TEXT NOT NULL,
		status            INTEGER NOT NULL,
		status_code       INTEGER NOT NULL,
		streaming         INTEGER NOT NULL,
		prompt_size       INTEGER NOT NULL,
		prompt_eval_count INTEGER,
		eval_count        INTEGER,
		eval_duration_ns  INTEGER,
		total_duration_ns INTEGER,
		load_duration_ns  INTEGER,
		per_sec           REAL,
		latency_ms        REAL,
		detail            TEXT NOT NULL
	);
	CREATE INDEX idx_inf_model    ON inferences(model);
	CREATE INDEX idx_inf_at       ON inferences(at);
	CREATE INDEX idx_inf_endpoint ON inferences(endpoint);
	CREATE INDEX idx_inf_model_at ON inferences(model, at);`,
}
