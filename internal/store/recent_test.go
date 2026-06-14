package store

import (
	"testing"
	"time"

	"ollama-telemetry/internal/activity"
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
