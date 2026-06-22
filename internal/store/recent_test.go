package store

import (
	"testing"
	"time"

	"dllm-network/internal/activity"
	"dllm-network/internal/telemetry/inference"
)

func TestRecentKeepsBoundedChronologicalHistory(t *testing.T) {
	t.Parallel()

	recent := NewRecent(2, 3)
	base := time.Date(2026, time.June, 14, 22, 10, 0, 0, time.UTC)

	recent.RecordSnapshot(Snapshot{ObservedAt: base, ConfirmedModel: "gemma3"})
	recent.RecordSnapshot(Snapshot{ObservedAt: base.Add(time.Minute), ConfirmedModel: "mistral"})
	recent.RecordSnapshot(Snapshot{ObservedAt: base.Add(2 * time.Minute), ConfirmedModel: "phi4"})

	recent.AppendActivity(activity.Event{Kind: activity.KindInferredModelLoaded, ObservedAt: base, Model: "gemma3"})
	recent.AppendActivity(activity.Event{Kind: activity.KindInferredModelChanged, ObservedAt: base.Add(time.Minute), Model: "mistral"})
	recent.AppendActivity(activity.Event{Kind: activity.KindInferredIdle, ObservedAt: base.Add(2 * time.Minute), Model: "mistral"})
	recent.AppendActivity(activity.Event{Kind: activity.KindInferredModelChanged, ObservedAt: base.Add(3 * time.Minute), Model: "phi4"})

	snapshots := recent.Snapshots()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots after bounding, got %d", len(snapshots))
	}
	if !snapshots[0].ObservedAt.Equal(base.Add(time.Minute)) || snapshots[0].ConfirmedModel != "mistral" {
		t.Fatalf("expected oldest retained snapshot to be mistral at +1m, got %+v", snapshots[0])
	}
	if !snapshots[1].ObservedAt.Equal(base.Add(2*time.Minute)) || snapshots[1].ConfirmedModel != "phi4" {
		t.Fatalf("expected newest retained snapshot to be phi4 at +2m, got %+v", snapshots[1])
	}

	activities := recent.Activities()
	if len(activities) != 3 {
		t.Fatalf("expected 3 activity events after bounding, got %d", len(activities))
	}
	if activities[0].Kind != activity.KindInferredModelChanged || !activities[0].ObservedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("expected oldest retained activity to be model changed at +1m, got %+v", activities[0])
	}
	if activities[2].Kind != activity.KindInferredModelChanged || activities[2].Model != "phi4" {
		t.Fatalf("expected newest retained activity to be phi4 model changed, got %+v", activities[2])
	}
}

// ---- task 4.9 RED: transition-aware snapshot dedup --------------------------

func TestStore_NoNewEntryOnUnchangedPoll(t *testing.T) {
	t.Parallel()

	recent := NewRecent(10, 10)
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	// First poll: model "llama3" begins running — this IS a transition.
	first := Snapshot{ObservedAt: base, ConfirmedModel: "llama3"}
	changed := recent.RecordSnapshotOnTransition(first)
	if !changed {
		t.Fatal("expected first snapshot to be recorded (transition from empty to llama3)")
	}

	// Second poll: same model still running — NOT a transition.
	second := Snapshot{ObservedAt: base.Add(5 * time.Second), ConfirmedModel: "llama3"}
	changed = recent.RecordSnapshotOnTransition(second)
	if changed {
		t.Fatal("expected no new entry appended when model unchanged across polls")
	}

	// Third poll: same model, another 5s later — still NOT a transition.
	third := Snapshot{ObservedAt: base.Add(10 * time.Second), ConfirmedModel: "llama3"}
	changed = recent.RecordSnapshotOnTransition(third)
	if changed {
		t.Fatal("expected no new entry appended on second identical poll")
	}

	snapshots := recent.Snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected exactly 1 snapshot after 3 identical-model polls, got %d", len(snapshots))
	}
	if snapshots[0].ConfirmedModel != "llama3" {
		t.Fatalf("expected retained snapshot to be llama3, got %q", snapshots[0].ConfirmedModel)
	}
}

// ---- task 4.11 RED: new entry on inference completion -----------------------

func TestStore_NewEntryOnInferenceCompletion(t *testing.T) {
	t.Parallel()

	recent := NewRecent(10, 10)
	base := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	// Seed a baseline: model running, already recorded.
	recent.RecordSnapshotOnTransition(Snapshot{ObservedAt: base, ConfirmedModel: "llama3"})

	// A done:true completion arrives — this IS a genuine event, distinct from a poll.
	completedInf := inference.Inference{
		At:       base.Add(30 * time.Second),
		Endpoint: "/api/generate",
		Method:   "POST",
		Model:    "llama3",
		Status:   inference.PhaseCompleted,
		Tokens: &inference.TokenStats{
			EvalCount: 48,
			PerSec:    20.0,
			LatencyMS: 2600.0,
		},
	}

	recent.RecordInferenceCompletion(completedInf)

	// The inference feed should have one entry reflecting the completion.
	events := recent.InferenceEvents()
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 inference event after one completion, got %d", len(events))
	}
	evt := events[0]
	if evt.Model != "llama3" {
		t.Errorf("expected inference event model=llama3, got %q", evt.Model)
	}
	if evt.Status != inference.PhaseCompleted {
		t.Errorf("expected inference event status=PhaseCompleted, got %v", evt.Status)
	}
	if evt.Tokens == nil || evt.Tokens.PerSec != 20.0 {
		t.Errorf("expected tokens_per_sec=20.0 in inference event, got %v", evt.Tokens)
	}
}

func TestRecentSupportsSafeEmptyStateAndLatestSnapshotReplacement(t *testing.T) {
	t.Parallel()

	recent := NewRecent(2, 2)

	if latest, ok := recent.LatestSnapshot(); ok {
		t.Fatalf("expected empty store to have no latest snapshot, got %+v", latest)
	}
	if snapshots := recent.Snapshots(); len(snapshots) != 0 {
		t.Fatalf("expected empty snapshots slice, got %d entries", len(snapshots))
	}
	if activities := recent.Activities(); len(activities) != 0 {
		t.Fatalf("expected empty activity slice, got %d entries", len(activities))
	}
	if replaced := recent.ReplaceLatestSnapshot(Snapshot{ConfirmedModel: "should-not-panic"}); replaced {
		t.Fatal("expected empty store replacement to report false")
	}

	firstObservedAt := time.Date(2026, time.June, 14, 22, 20, 0, 0, time.UTC)
	recent.RecordSnapshot(Snapshot{ObservedAt: firstObservedAt, ConfirmedModel: "gemma3"})

	replaced := recent.ReplaceLatestSnapshot(Snapshot{ObservedAt: firstObservedAt.Add(30 * time.Second), ConfirmedModel: "mistral"})
	if !replaced {
		t.Fatal("expected latest snapshot replacement to succeed")
	}

	latest, ok := recent.LatestSnapshot()
	if !ok {
		t.Fatal("expected latest snapshot after replacement")
	}
	if latest.ConfirmedModel != "mistral" {
		t.Fatalf("expected replaced latest model mistral, got %+v", latest)
	}
	if !latest.ObservedAt.Equal(firstObservedAt.Add(30 * time.Second)) {
		t.Fatalf("expected replacement observed_at to update, got %+v", latest)
	}
	if snapshots := recent.Snapshots(); len(snapshots) != 1 {
		t.Fatalf("expected replacement to keep a single snapshot entry, got %d", len(snapshots))
	}
}
