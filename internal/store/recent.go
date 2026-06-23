// Package store holds the bounded chronological history of confirmed-model
// snapshots and inferred-activity events used by the dashboard projector.
//
// Transition-aware API (WU4, 2026-06-16):
//   - RecordSnapshotOnTransition: only appends when the confirmed model name
//     changes, preventing repeated near-duplicate rows from poll cycles.
//   - RecordInferenceCompletion: appends one entry to the inference-event feed
//     per done:true completion — always a genuine state change.
//   - InferenceEvents: returns the bounded inference event history.
//
// The original RecordSnapshot method is preserved for backward compatibility.
package store

import (
	"sync"
	"time"

	"dllm-network/internal/activity"
	"dllm-network/internal/telemetry/inference"
)

type Snapshot struct {
	ObservedAt     time.Time `json:"observedAt"`
	ConfirmedModel string    `json:"confirmedModel,omitempty"`
}

type Recent struct {
	mu             sync.RWMutex
	snapshotLimit  int
	activityLimit  int
	inferenceLimit int
	snapshots      []Snapshot
	activities     []activity.Event
	inferences     []inference.Inference
}

func NewRecent(snapshotLimit, activityLimit int) *Recent {
	return &Recent{
		snapshotLimit:  normalizeLimit(snapshotLimit),
		activityLimit:  normalizeLimit(activityLimit),
		inferenceLimit: normalizeLimit(snapshotLimit), // same capacity as snapshots
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

// RecordSnapshotOnTransition appends snapshot only when the confirmed model
// name differs from the most recently recorded snapshot. Returns true when a
// new entry was appended (a genuine transition occurred), false when the poll
// produced no change and the entry was suppressed.
func (recent *Recent) RecordSnapshotOnTransition(snapshot Snapshot) bool {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	// Only append when the model name has actually changed.
	if len(recent.snapshots) > 0 {
		last := recent.snapshots[len(recent.snapshots)-1]
		if last.ConfirmedModel == snapshot.ConfirmedModel {
			return false
		}
	}

	recent.snapshots = appendBoundedSnapshot(recent.snapshots, snapshot, recent.snapshotLimit)
	return true
}

// recordInferenceEventLocked appends an inference event to the bounded feed.
// The caller must hold recent.mu. Used by both completion and cancellation
// wrappers since the append logic is identical.
func (recent *Recent) recordInferenceEventLocked(inf inference.Inference) {
	recent.inferences = appendBoundedInference(recent.inferences, inf, recent.inferenceLimit)
}

// RecordInferenceCompletion appends a completed inference event to the bounded
// inference-event feed. Each completion is always a genuine state transition —
// no dedup is applied. Callers should only call this for done:true events.
func (recent *Recent) RecordInferenceCompletion(inf inference.Inference) {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	recent.recordInferenceEventLocked(inf)
}

// RecordInferenceCancellation appends a cancelled/incomplete inference to the
// bounded terminal-event feed. The feed holds all TERMINAL inference events;
// a cancellation is terminal too (the request will never complete). Callers
// should only call this once per connection, when an in-progress request is
// evicted without ever completing. The event is always stored with
// Status == PhaseCancelled so the projection layer cannot mistake it for a
// successful completion.
func (recent *Recent) RecordInferenceCancellation(inf inference.Inference) {
	recent.mu.Lock()
	defer recent.mu.Unlock()

	inf.Status = inference.PhaseCancelled
	recent.recordInferenceEventLocked(inf)
}

// InferenceEvents returns a copy of the bounded inference-event history in
// chronological order. Returns nil when no completions have been recorded.
func (recent *Recent) InferenceEvents() []inference.Inference {
	recent.mu.RLock()
	defer recent.mu.RUnlock()

	if len(recent.inferences) == 0 {
		return nil
	}
	return append([]inference.Inference(nil), recent.inferences...)
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

func appendBoundedInference(existing []inference.Inference, inf inference.Inference, limit int) []inference.Inference {
	if limit == 0 {
		return nil
	}

	existing = append(existing, inf)
	if len(existing) <= limit {
		return existing
	}

	return append([]inference.Inference(nil), existing[len(existing)-limit:]...)
}
