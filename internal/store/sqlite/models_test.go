package sqlite

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

// TestStore_Models_DistinctNoDuplicates (spec: "Distinct models list") seeds
// 5 llama3 + 2 mistral rows and asserts Models returns exactly the 2
// distinct names, no duplicates.
func TestStore_Models_DistinctNoDuplicates(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	var seed []inference.Inference
	for i := 0; i < 5; i++ {
		seed = append(seed, fixtureAt(
			"llama-"+time.Duration(i).String(), base.Add(time.Duration(i)*time.Second),
			"llama3", "/api/generate", 100, 50))
	}
	for i := 0; i < 2; i++ {
		seed = append(seed, fixtureAt(
			"mistral-"+time.Duration(i).String(), base.Add(time.Duration(i)*time.Second),
			"mistral", "/api/generate", 100, 50))
	}
	st := openSeededStore(t, seed)

	got, err := st.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}

	seen := map[string]bool{}
	for _, m := range got {
		if seen[m] {
			t.Fatalf("duplicate model in result: %q (full result: %v)", m, got)
		}
		seen[m] = true
	}
	if len(got) != 2 || !seen["llama3"] || !seen["mistral"] {
		t.Fatalf("expected exactly [llama3 mistral] (any order), got %v", got)
	}
}

// TestStore_Models_EmptyStore returns an empty (not nil-panicking, not
// error) result when no inferences have been persisted.
func TestStore_Models_EmptyStore(t *testing.T) {
	t.Parallel()

	st := openSeededStore(t, nil)

	got, err := st.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 models, got %v", got)
	}
}
