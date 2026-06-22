package sqlite

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

func TestStore_ResolveInferenceContext_EmptyDataset(t *testing.T) {
	t.Parallel()

	st := openSeededStore(t, nil)

	got, err := st.ResolveInferenceContext(context.Background())
	if err != nil {
		t.Fatalf("ResolveInferenceContext: %v", err)
	}
	if got.Counts.Total != 0 {
		t.Fatalf("Counts.Total: got %d, want 0", got.Counts.Total)
	}
	if len(got.SupportedFilters) != 5 {
		t.Fatalf("SupportedFilters: got %d entries, want 5", len(got.SupportedFilters))
	}
	if got.TimeRange.Oldest != nil || got.TimeRange.Latest != nil {
		t.Fatalf("expected nil time bounds for empty dataset, got %+v", got.TimeRange)
	}
}

func TestStore_ResolveInferenceContext_AggregatesAvailableUniverse(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 19, 10, 0, 0, 0, time.UTC)
	seed := []inference.Inference{
		fixtureAt("c", base.Add(3*time.Minute), "llama3", "/api/generate", 120, 60),
		fixtureAt("b", base.Add(2*time.Minute), "llama3", "/api/chat", 110, 55),
		fixtureAt("a", base.Add(1*time.Minute), "mistral", "/api/generate", 100, 50),
	}
	seed[1].Status = inference.PhaseCancelled
	st := openSeededStore(t, seed)

	got, err := st.ResolveInferenceContext(context.Background())
	if err != nil {
		t.Fatalf("ResolveInferenceContext: %v", err)
	}
	if got.Counts.Total != 3 {
		t.Fatalf("Counts.Total: got %d, want 3", got.Counts.Total)
	}
	if len(got.Models) != 2 || got.Models[0].Value != "llama3" || got.Models[0].Count != 2 {
		t.Fatalf("Models: got %+v", got.Models)
	}
	if len(got.Endpoints) != 2 {
		t.Fatalf("Endpoints: got %+v", got.Endpoints)
	}
	if len(got.Statuses) != 2 {
		t.Fatalf("Statuses: got %+v", got.Statuses)
	}
	if got.TimeRange.Oldest == nil || !got.TimeRange.Oldest.Equal(base.Add(1*time.Minute)) {
		t.Fatalf("Oldest: got %v", got.TimeRange.Oldest)
	}
	if got.TimeRange.Latest == nil || !got.TimeRange.Latest.Equal(base.Add(3*time.Minute)) {
		t.Fatalf("Latest: got %v", got.TimeRange.Latest)
	}
}

func TestStore_SearchInferences_StableCursorWithoutDuplicates(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 19, 10, 0, 0, 0, time.UTC)
	seed := []inference.Inference{
		fixtureAt("inf-a", base, "llama3", "/api/generate", 100, 50),
		fixtureAt("inf-c", base, "llama3", "/api/generate", 110, 55),
		fixtureAt("inf-b", base, "llama3", "/api/generate", 120, 60),
		fixtureAt("older", base.Add(-time.Minute), "llama3", "/api/generate", 90, 45),
	}
	st := openSeededStore(t, seed)

	page1, err := st.SearchInferences(context.Background(), store.SearchInferencesQuery{Model: "llama3", Limit: 2})
	if err != nil {
		t.Fatalf("SearchInferences page1: %v", err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("page1 items: got %d, want 2", len(page1.Items))
	}
	if page1.Items[0].ID != "inf-c" || page1.Items[1].ID != "inf-b" {
		t.Fatalf("page1 order: got [%s %s]", page1.Items[0].ID, page1.Items[1].ID)
	}
	if page1.NextCursor == "" {
		t.Fatal("expected next cursor for first page")
	}

	page2, err := st.SearchInferences(context.Background(), store.SearchInferencesQuery{Model: "llama3", Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("SearchInferences page2: %v", err)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("page2 items: got %d, want 2", len(page2.Items))
	}
	if page2.Items[0].ID != "inf-a" || page2.Items[1].ID != "older" {
		t.Fatalf("page2 order: got [%s %s]", page2.Items[0].ID, page2.Items[1].ID)
	}
	if page2.NextCursor != "" {
		t.Fatalf("expected final page to have empty cursor, got %q", page2.NextCursor)
	}
}

func TestStore_SearchInferences_NoMatchesReturnsEmptyPage(t *testing.T) {
	t.Parallel()

	st := openSeededStore(t, []inference.Inference{
		fixtureAt("only", time.Now().UTC(), "llama3", "/api/generate", 100, 50),
	})

	got, err := st.SearchInferences(context.Background(), store.SearchInferencesQuery{Model: "missing", Limit: 5})
	if err != nil {
		t.Fatalf("SearchInferences: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("expected empty page, got %d items", len(got.Items))
	}
	if got.NextCursor != "" {
		t.Fatalf("expected empty next cursor, got %q", got.NextCursor)
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
