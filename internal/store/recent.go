package store

import (
	"sync"
	"time"

	"ollama-telemetry/internal/activity"
)

type Snapshot struct {
	ObservedAt     time.Time `json:"observedAt"`
	ConfirmedModel string    `json:"confirmedModel,omitempty"`
}

type Recent struct {
	mu            sync.RWMutex
	snapshotLimit int
	activityLimit int
	snapshots     []Snapshot
	activities    []activity.Event
}

func NewRecent(snapshotLimit, activityLimit int) *Recent {
	return &Recent{
		snapshotLimit: normalizeLimit(snapshotLimit),
		activityLimit: normalizeLimit(activityLimit),
	}
}

func (recent *Recent) RecordSnapshot(snapshot Snapshot) {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	recent.snapshots = appendBoundedSnapshot(recent.snapshots, snapshot, recent.snapshotLimit)
}

func (recent *Recent) ReplaceLatestSnapshot(snapshot Snapshot) bool {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	if len(recent.snapshots) == 0 {
		return false
	}

	recent.snapshots[len(recent.snapshots)-1] = snapshot
	return true
}

func (recent *Recent) LatestSnapshot() (Snapshot, bool) {
	recent.mu.RLock()
	defer recent.mu.RUnlock()

	if len(recent.snapshots) == 0 {
		return Snapshot{}, false
	}

	return recent.snapshots[len(recent.snapshots)-1], true
}

func (recent *Recent) Snapshots() []Snapshot {
	recent.mu.RLock()
	defer recent.mu.RUnlock()

	return append([]Snapshot(nil), recent.snapshots...)
}

func (recent *Recent) AppendActivity(event activity.Event) {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	recent.activities = appendBoundedActivity(recent.activities, event, recent.activityLimit)
}

func (recent *Recent) Activities() []activity.Event {
	recent.mu.RLock()
	defer recent.mu.RUnlock()

	return append([]activity.Event(nil), recent.activities...)
}

func normalizeLimit(limit int) int {
	if limit < 0 {
		return 0
	}

	return limit
}

func appendBoundedSnapshot(existing []Snapshot, snapshot Snapshot, limit int) []Snapshot {
	if limit == 0 {
		return nil
	}

	existing = append(existing, snapshot)
	if len(existing) <= limit {
		return existing
	}

	return append([]Snapshot(nil), existing[len(existing)-limit:]...)
}

func appendBoundedActivity(existing []activity.Event, event activity.Event, limit int) []activity.Event {
	if limit == 0 {
		return nil
	}

	existing = append(existing, event)
	if len(existing) <= limit {
		return existing
	}

	return append([]activity.Event(nil), existing[len(existing)-limit:]...)
}
