package store

import (
	"context"
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

// Filter narrows a Query or Stats call by any combination of model,
// endpoint, status, and a [Since, Until) time-window, with a result Limit.
// A zero value for any field disables that constraint: Model=="" matches
// any model, Since/Until zero Time disables that bound, Limit<=0 disables
// the cap (Query) or is ignored (Stats).
//
// Status uses *inference.Phase (not inference.Phase) so "no status filter"
// (nil) is distinguishable from "filter to PhaseInProgress" (status==0,
// the Phase zero value) — a plain Phase field could not express both.
type Filter struct {
	Model    string
	Endpoint string
	Status   *inference.Phase
	Since    time.Time
	Until    time.Time
	Limit    int
}

// ModelStats is the per-model slice of an aggregate Stats result.
type ModelStats struct {
	Model string
	Count int
}

// Stats is the aggregate result of a Stats call: tokens/sec and latency
// percentiles over the filtered dataset, plus per-model counts. All
// percentile/latency fields are zero (not an error) when the filtered
// dataset is empty or contains no rows with non-nil Tokens.
type Stats struct {
	Count        int
	PerSecP50    float64
	PerSecP95    float64
	LatencyMSP50 float64
	LatencyMSP95 float64
	ByModel      []ModelStats
}

// InferenceWriter is the narrow write-side port (design D5). Implemented by
// sqlite.Store; consumed by internal/persistence.
type InferenceWriter interface {
	Save(ctx context.Context, infs []inference.Inference) error
}

// InferenceReader is the read-side query port (design D5) the MCP core
// (slice 4) will depend on exclusively. Implemented by sqlite.Store.
//
// Deliberately segregated from InferenceWriter (ISP) and from the live
// in-memory Recent ring, which stays a separate, uncoupled projection — see
// design D5's rationale.
type InferenceReader interface {
	// ResolveInferenceContext returns lightweight discovery data for the stored
	// inference universe, without bodies or per-event detail.
	ResolveInferenceContext(ctx context.Context) (ResolveInferenceContextResult, error)

	// SearchInferences returns lightweight summaries ordered newest-first using
	// a stable total ordering and an opaque cursor.
	SearchInferences(ctx context.Context, query SearchInferencesQuery) (SearchInferencesResult, error)

	// GetInferenceContext returns one selected inference's bounded context for
	// the staged MCP detail flow.
	GetInferenceContext(ctx context.Context, query GetInferenceContextQuery) (GetInferenceContextResult, bool, error)

	// Query lists full inference records for non-MCP consumers that still need
	// the legacy durable-read contract.
	Query(ctx context.Context, filter Filter) ([]inference.Inference, error)

	// Get fetches one full record by id for non-MCP consumers such as the GUI
	// detail binding. Returns ok=false (no error) when id is unknown.
	Get(ctx context.Context, id string) (inference.Inference, bool, error)

	// Stats computes aggregate metrics for non-MCP readers that still need the
	// legacy aggregate contract.
	Stats(ctx context.Context, filter Filter) (Stats, error)

	// Models lists the distinct model names for non-MCP readers that still need
	// the legacy model index contract.
	Models(ctx context.Context) ([]string, error)
}
