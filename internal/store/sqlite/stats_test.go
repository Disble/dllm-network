package sqlite

import (
	"context"
	"math"
	"sort"
	"testing"
	"time"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// TestStore_Stats_OverPopulatedDataset (spec: "Stats over a populated
// dataset") seeds 10 completed inferences across 2 models with known PerSec
// and LatencyMS values, computes expected p50/p95 with a reference
// nearest-rank implementation, and asserts Stats matches plus that
// per-model counts sum to N.
func TestStore_Stats_OverPopulatedDataset(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	perSecs := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	latencies := []float64{100, 90, 80, 70, 60, 50, 40, 30, 20, 10}

	var seed []inference.Inference
	for i, ps := range perSecs {
		model := "llama3"
		if i%3 == 0 {
			model = "mistral"
		}
		seed = append(seed, fixtureAt(
			"inf-"+time.Duration(i).String(), base.Add(time.Duration(i)*time.Second),
			model, "/api/generate", ps, latencies[i]))
	}
	st := openSeededStore(t, seed)

	got, err := st.Stats(context.Background(), store.Filter{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if got.Count != len(seed) {
		t.Fatalf("Count: got %d, want %d", got.Count, len(seed))
	}

	wantPerSecP50 := nearestRankPercentile(perSecs, 0.50)
	wantPerSecP95 := nearestRankPercentile(perSecs, 0.95)
	wantLatencyP50 := nearestRankPercentile(latencies, 0.50)
	wantLatencyP95 := nearestRankPercentile(latencies, 0.95)

	if got.PerSecP50 != wantPerSecP50 {
		t.Errorf("PerSecP50: got %v, want %v", got.PerSecP50, wantPerSecP50)
	}
	if got.PerSecP95 != wantPerSecP95 {
		t.Errorf("PerSecP95: got %v, want %v", got.PerSecP95, wantPerSecP95)
	}
	if got.LatencyMSP50 != wantLatencyP50 {
		t.Errorf("LatencyMSP50: got %v, want %v", got.LatencyMSP50, wantLatencyP50)
	}
	if got.LatencyMSP95 != wantLatencyP95 {
		t.Errorf("LatencyMSP95: got %v, want %v", got.LatencyMSP95, wantLatencyP95)
	}

	sum := 0
	byModel := map[string]int{}
	for _, ms := range got.ByModel {
		sum += ms.Count
		byModel[ms.Model] = ms.Count
	}
	if sum != len(seed) {
		t.Fatalf("ByModel counts sum: got %d, want %d (ByModel=%+v)", sum, len(seed), got.ByModel)
	}
	if byModel["mistral"] != 4 || byModel["llama3"] != 6 {
		t.Fatalf("expected mistral=4 llama3=6, got %+v", byModel)
	}
}

// TestStore_Stats_OverEmptyDataset (spec: "Stats over empty dataset")
// asserts zero counts without error, not a panic or division-by-zero NaN.
func TestStore_Stats_OverEmptyDataset(t *testing.T) {
	t.Parallel()

	st := openSeededStore(t, nil)

	got, err := st.Stats(context.Background(), store.Filter{})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.Count != 0 {
		t.Fatalf("Count: got %d, want 0", got.Count)
	}
	if got.PerSecP50 != 0 || got.PerSecP95 != 0 || got.LatencyMSP50 != 0 || got.LatencyMSP95 != 0 {
		t.Fatalf("expected all-zero percentiles for empty dataset, got %+v", got)
	}
	if len(got.ByModel) != 0 {
		t.Fatalf("expected empty ByModel, got %+v", got.ByModel)
	}
}

// TestStore_Stats_RespectsFilter verifies Stats applies the same Filter
// semantics as Query (model scoping here), not just a global aggregate.
func TestStore_Stats_RespectsFilter(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	seed := []inference.Inference{
		fixtureAt("a", base, "llama3", "/api/generate", 100, 50),
		fixtureAt("b", base, "llama3", "/api/generate", 200, 60),
		fixtureAt("c", base, "mistral", "/api/generate", 999, 999),
	}
	st := openSeededStore(t, seed)

	got, err := st.Stats(context.Background(), store.Filter{Model: "llama3"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got.Count != 2 {
		t.Fatalf("Count: got %d, want 2", got.Count)
	}
	if len(got.ByModel) != 1 || got.ByModel[0].Model != "llama3" || got.ByModel[0].Count != 2 {
		t.Fatalf("expected ByModel=[{llama3 2}], got %+v", got.ByModel)
	}
}

// nearestRankPercentile is a reference implementation (nearest-rank method,
// ceil(p*N)) used only by tests to compute the expected percentile
// independently from the implementation under test.
func nearestRankPercentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	rank := int(math.Ceil(p * float64(len(sorted))))
	idx := rank - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
