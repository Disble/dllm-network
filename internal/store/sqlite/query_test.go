package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// openSeededStore opens a fresh temp-dir Store and saves seed, returning the
// store for read-side tests (slice 3). Extracted so query_test.go and
// stats_test.go share one seeding helper instead of duplicating Open/Save
// boilerplate (no-duplication convention).
func openSeededStore(t *testing.T, seed []inference.Inference) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if len(seed) > 0 {
		if err := st.Save(context.Background(), seed); err != nil {
			t.Fatalf("Save seed: %v", err)
		}
	}
	return st
}

// fixtureAt builds a minimal-but-valid completed inference fixture at a
// given time offset from a fixed base, varying model/endpoint as requested.
// Shared across query_test.go and stats_test.go to avoid re-deriving the
// same boilerplate per test (no-duplication convention).
func fixtureAt(id string, at time.Time, model, endpoint string, perSec, latencyMS float64) inference.Inference {
	return inference.Inference{
		ID:         id,
		At:         at,
		Endpoint:   endpoint,
		Method:     "POST",
		Model:      model,
		PromptSize: 10,
		Status:     inference.PhaseCompleted,
		StatusCode: 200,
		Tokens: &inference.TokenStats{
			PromptEvalCount: 5,
			EvalCount:       50,
			EvalDuration:    time.Duration(latencyMS) * time.Millisecond,
			TotalDuration:   time.Duration(latencyMS) * time.Millisecond,
			PerSec:          perSec,
			LatencyMS:       latencyMS,
		},
	}
}

// TestStore_Query_FilterByModelAndTimeWindowWithLimit (spec: "Filter by model
// and time-window with limit") seeds inferences across two models and a
// spread of timestamps, then asserts Query returns only the matching model
// within the window, capped at the limit, ordered most-recent-first.
func TestStore_Query_FilterByModelAndTimeWindowWithLimit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	seed := []inference.Inference{
		fixtureAt("llama-1", base.Add(1*time.Minute), "llama3", "/api/generate", 100, 50),
		fixtureAt("llama-2", base.Add(2*time.Minute), "llama3", "/api/generate", 110, 55),
		fixtureAt("llama-3", base.Add(3*time.Minute), "llama3", "/api/generate", 120, 60),
		fixtureAt("llama-too-early", base.Add(-1*time.Hour), "llama3", "/api/generate", 90, 45),
		fixtureAt("mistral-1", base.Add(2*time.Minute), "mistral", "/api/generate", 200, 30),
	}
	st := openSeededStore(t, seed)

	filter := store.Filter{
		Model: "llama3",
		Since: base,
		Until: base.Add(10 * time.Minute),
		Limit: 2,
	}
	got, err := st.Query(context.Background(), filter)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results (limit), got %d: %+v", len(got), got)
	}
	if got[0].ID != "llama-3" || got[1].ID != "llama-2" {
		t.Fatalf("expected most-recent-first order [llama-3, llama-2], got [%s, %s]", got[0].ID, got[1].ID)
	}
	for _, inf := range got {
		if inf.Model != "llama3" {
			t.Errorf("unexpected model in results: %q", inf.Model)
		}
	}
}

// TestStore_Query_NoMatches_ReturnsEmptyNotError (spec: "No matches returns
// empty, not error").
func TestStore_Query_NoMatches_ReturnsEmptyNotError(t *testing.T) {
	t.Parallel()

	st := openSeededStore(t, []inference.Inference{
		fixtureAt("only-one", time.Now().UTC(), "llama3", "/api/generate", 100, 50),
	})

	got, err := st.Query(context.Background(), store.Filter{Model: "does-not-exist"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 results, got %d", len(got))
	}
}

// TestStore_Query_FilterByEndpointAndStatus exercises the endpoint and
// status arms of Filter independently of the model arm covered above.
func TestStore_Query_FilterByEndpointAndStatus(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	inProgress := inference.PhaseInProgress
	seed := []inference.Inference{
		fixtureAt("gen-1", base, "llama3", "/api/generate", 100, 50),
		{
			ID:       "tags-1",
			At:       base,
			Endpoint: "/api/tags",
			Method:   "GET",
			Model:    "",
			Status:   inference.PhaseInProgress,
		},
	}
	st := openSeededStore(t, seed)

	byEndpoint, err := st.Query(context.Background(), store.Filter{Endpoint: "/api/tags"})
	if err != nil {
		t.Fatalf("Query by endpoint: %v", err)
	}
	if len(byEndpoint) != 1 || byEndpoint[0].ID != "tags-1" {
		t.Fatalf("expected exactly [tags-1], got %+v", byEndpoint)
	}

	byStatus, err := st.Query(context.Background(), store.Filter{Status: &inProgress})
	if err != nil {
		t.Fatalf("Query by status: %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].ID != "tags-1" {
		t.Fatalf("expected exactly [tags-1], got %+v", byStatus)
	}
}

// TestStore_Query_NoLimit_ReturnsAllMatches verifies Limit<=0 disables the
// cap rather than returning zero rows.
func TestStore_Query_NoLimit_ReturnsAllMatches(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	seed := []inference.Inference{
		fixtureAt("a", base.Add(1*time.Minute), "llama3", "/api/generate", 100, 50),
		fixtureAt("b", base.Add(2*time.Minute), "llama3", "/api/generate", 100, 50),
		fixtureAt("c", base.Add(3*time.Minute), "llama3", "/api/generate", 100, 50),
	}
	st := openSeededStore(t, seed)

	got, err := st.Query(context.Background(), store.Filter{Model: "llama3"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results with no limit, got %d", len(got))
	}
}
